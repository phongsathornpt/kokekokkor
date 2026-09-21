package probing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

// DefaultProbeTimeout is the maximum duration for a model test probe.
const DefaultProbeTimeout = 12 * time.Second

// Service probes upstream providers to verify model connectivity and latency.
type Service struct {
	client  upstream.Client
	timeout time.Duration
}

// NewService creates a new model probing service.
func NewService(client upstream.Client) *Service {
	return &Service{
		client:  client,
		timeout: DefaultProbeTimeout,
	}
}

// SetTimeout sets a custom timeout for test probes.
func (s *Service) SetTimeout(d time.Duration) {
	s.timeout = d
}

// TestModel performs an end-to-end probe for a specific model under a provider target.
func (s *Service) TestModel(ctx context.Context, target provider.Target, modelID string) TestResult {
	if s.client == nil {
		return TestResult{
			OK:           false,
			ErrorMessage: "upstream client not configured",
		}
	}

	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return TestResult{
			OK:           false,
			ErrorMessage: "model ID must not be empty",
		}
	}

	timeout := s.timeout
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	req, err := s.buildProbeRequest(target, modelID)
	if err != nil {
		return TestResult{
			OK:           false,
			Latency:      time.Since(start),
			LatencyMs:    time.Since(start).Milliseconds(),
			ErrorMessage: fmt.Sprintf("build probe request: %v", err),
		}
	}

	resp, err := s.client.Do(ctx, target, req)
	latency := time.Since(start)
	latencyMs := latency.Milliseconds()

	if err != nil {
		return TestResult{
			OK:           false,
			Latency:      latency,
			LatencyMs:    latencyMs,
			ErrorMessage: err.Error(),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := extractErrorMessage(resp.Body, resp.StatusCode)
		return TestResult{
			OK:           false,
			Latency:      latency,
			LatencyMs:    latencyMs,
			StatusCode:   resp.StatusCode,
			ErrorMessage: errMsg,
		}
	}

	snippet := extractResponseSnippet(resp.Body)
	return TestResult{
		OK:         true,
		Latency:    latency,
		LatencyMs:  latencyMs,
		StatusCode: resp.StatusCode,
		Snippet:    snippet,
	}
}

func (s *Service) buildProbeRequest(target provider.Target, modelID string) (upstream.Request, error) {
	if target.IsAntigravity() {
		// Antigravity private Google Cloud Code endpoint
		wirePayload := []byte(`{
			"contents": [{"role": "user", "parts": [{"text": "hi"}]}],
			"generationConfig": {"maxOutputTokens": 1024}
		}`)
		formatted, err := gemini.FormatAntigravityRequest(wirePayload, modelID, "")
		if err != nil {
			return upstream.Request{}, fmt.Errorf("format Antigravity request: %w", err)
		}
		return upstream.Request{
			Method: http.MethodPost,
			Path:   gemini.AntigravityGenerateContentPath(),
			Header: gemini.AntigravityHeaders(http.Header{}),
			Body:   formatted,
		}, nil
	}

	switch target.EffectiveProtocol() {
	case provider.ProtocolAnthropic:
		body := map[string]any{
			"model":      modelID,
			"max_tokens": 1024,
			"messages": []map[string]string{
				{"role": "user", "content": "hi"},
			},
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return upstream.Request{}, err
		}
		return upstream.Request{
			Method: http.MethodPost,
			Path:   "/v1/messages",
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   encoded,
		}, nil

	case provider.ProtocolGemini:
		path, err := gemini.GenerateContentPath(modelID)
		if err != nil {
			return upstream.Request{}, err
		}
		wirePayload := []byte(`{
			"contents": [{"role": "user", "parts": [{"text": "hi"}]}],
			"generationConfig": {"maxOutputTokens": 1024}
		}`)
		return upstream.Request{
			Method: http.MethodPost,
			Path:   path,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   wirePayload,
		}, nil

	default: // ProtocolOpenAI
		path := openAIChatPath(target.BaseURL)
		body := map[string]any{
			"model":      modelID,
			"max_tokens": 1024,
			"messages": []map[string]string{
				{"role": "user", "content": "hi"},
			},
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return upstream.Request{}, err
		}
		return upstream.Request{
			Method: http.MethodPost,
			Path:   path,
			Header: http.Header{"Content-Type": {"application/json"}},
			Body:   encoded,
		}, nil
	}
}

func openAIChatPath(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return "/chat/completions"
	}
	return "/v1/chat/completions"
}

func extractErrorMessage(body []byte, statusCode int) string {
	if len(body) == 0 {
		return fmt.Sprintf("HTTP %d %s", statusCode, http.StatusText(statusCode))
	}

	var parsed struct {
		Error any    `json:"error"`
		Msg   string `json:"msg"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Error != nil {
			switch val := parsed.Error.(type) {
			case string:
				if val != "" {
					return val
				}
			case map[string]any:
				if msg, ok := val["message"].(string); ok && msg != "" {
					return msg
				}
				if status, ok := val["status"].(string); ok && status != "" {
					return status
				}
			}
		}
		if parsed.Msg != "" {
			return parsed.Msg
		}
	}

	raw := string(body)
	if len(raw) > 160 {
		raw = raw[:160] + "..."
	}
	return fmt.Sprintf("HTTP %d: %s", statusCode, strings.TrimSpace(raw))
}

func extractResponseSnippet(body []byte) string {
	if len(body) == 0 {
		return "ok"
	}

	// Try Gemini / Antigravity format
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &geminiResp); err == nil && len(geminiResp.Candidates) > 0 {
		for _, part := range geminiResp.Candidates[0].Content.Parts {
			if text := strings.TrimSpace(part.Text); text != "" {
				if len(text) > 60 {
					return text[:60] + "..."
				}
				return text
			}
		}
		return "ok"
	}

	// Try Anthropic format
	var anthropicResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &anthropicResp); err == nil && len(anthropicResp.Content) > 0 {
		for _, item := range anthropicResp.Content {
			if text := strings.TrimSpace(item.Text); text != "" {
				if len(text) > 60 {
					return text[:60] + "..."
				}
				return text
			}
		}
		return "ok"
	}

	// Try OpenAI format
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Choices) > 0 {
		text := strings.TrimSpace(openAIResp.Choices[0].Message.Content)
		if text != "" {
			if len(text) > 60 {
				return text[:60] + "..."
			}
			return text
		}
	}

	return "ok"
}
