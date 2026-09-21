package probing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

type mockUpstreamClient struct {
	doFunc func(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error)
}

func (m *mockUpstreamClient) Do(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error) {
	if m.doFunc != nil {
		return m.doFunc(ctx, target, req)
	}
	return upstream.Response{StatusCode: http.StatusOK}, nil
}

func (m *mockUpstreamClient) Stream(ctx context.Context, target provider.Target, req upstream.Request) (upstream.StreamResponse, error) {
	return upstream.StreamResponse{}, errors.New("stream not supported in mock")
}

func TestProbeAntigravityModelSuccess(t *testing.T) {
	target := provider.Target{
		ID:       "antigravity",
		Protocol: provider.ProtocolGemini,
		BaseURL:  "https://cloudcode-pa.googleapis.com",
	}

	var capturedReq upstream.Request
	client := &mockUpstreamClient{
		doFunc: func(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error) {
			capturedReq = req
			return upstream.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"candidates": [{
						"content": {
							"parts": [{"text": "Hello, I am Claude Opus running on Antigravity."}],
							"role": "model"
						},
						"finishReason": "STOP"
					}]
				}`),
			}, nil
		},
	}

	service := NewService(client)
	res := service.TestModel(context.Background(), target, "claude-opus-4-6-thinking")

	if !res.OK {
		t.Fatalf("expected OK test result, got error: %s", res.ErrorMessage)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", res.StatusCode)
	}
	if !strings.Contains(res.Snippet, "Hello, I am Claude Opus") {
		t.Errorf("expected snippet containing response text, got %q", res.Snippet)
	}

	// Verify request path & headers
	if capturedReq.Path != "/v1internal:generateContent" {
		t.Errorf("expected Antigravity path /v1internal:generateContent, got %s", capturedReq.Path)
	}
	if capturedReq.Header.Get("X-Client-Name") != "antigravity" {
		t.Errorf("expected X-Client-Name antigravity, got %s", capturedReq.Header.Get("X-Client-Name"))
	}
	if capturedReq.Header.Get("User-Agent") != "antigravity/1.107.0 darwin/arm64" {
		t.Errorf("expected User-Agent antigravity/1.107.0 darwin/arm64, got %s", capturedReq.Header.Get("User-Agent"))
	}

	// Verify request payload formatting
	var payload map[string]any
	if err := json.Unmarshal(capturedReq.Body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["model"] != "claude-opus-4-6-thinking" {
		t.Errorf("model = %v, want claude-opus-4-6-thinking", payload["model"])
	}
	if ideReqID, ok := payload["ideRequestId"].(string); !ok || !strings.HasPrefix(ideReqID, "agent/") {
		t.Errorf("ideRequestId = %v, want agent/ prefix", payload["ideRequestId"])
	}
}

func TestProbeOpenAIModelSuccess(t *testing.T) {
	target := provider.Target{
		ID:       "openai",
		Protocol: provider.ProtocolOpenAI,
		BaseURL:  "https://api.openai.com/v1",
	}

	var capturedReq upstream.Request
	client := &mockUpstreamClient{
		doFunc: func(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error) {
			capturedReq = req
			return upstream.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"choices": [{
						"message": {"role": "assistant", "content": "Hello world!"}
					}]
				}`),
			}, nil
		},
	}

	service := NewService(client)
	res := service.TestModel(context.Background(), target, "gpt-4o")

	if !res.OK {
		t.Fatalf("expected OK test result, got error: %s", res.ErrorMessage)
	}
	if capturedReq.Path != "/chat/completions" {
		t.Errorf("expected path /chat/completions, got %s", capturedReq.Path)
	}
	if res.Snippet != "Hello world!" {
		t.Errorf("snippet = %q, want Hello world!", res.Snippet)
	}
}

func TestProbeAnthropicModelSuccess(t *testing.T) {
	target := provider.Target{
		ID:       "anthropic",
		Protocol: provider.ProtocolAnthropic,
		BaseURL:  "https://api.anthropic.com",
	}

	var capturedReq upstream.Request
	client := &mockUpstreamClient{
		doFunc: func(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error) {
			capturedReq = req
			return upstream.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"content": [{"type": "text", "text": "Hi from Claude"}]
				}`),
			}, nil
		},
	}

	service := NewService(client)
	res := service.TestModel(context.Background(), target, "claude-3-7-sonnet-20250219")

	if !res.OK {
		t.Fatalf("expected OK, got: %s", res.ErrorMessage)
	}
	if capturedReq.Path != "/v1/messages" {
		t.Errorf("expected path /v1/messages, got %s", capturedReq.Path)
	}
	if res.Snippet != "Hi from Claude" {
		t.Errorf("snippet = %q, want Hi from Claude", res.Snippet)
	}
}

func TestProbeGeminiModelSuccess(t *testing.T) {
	target := provider.Target{
		ID:       "gemini",
		Protocol: provider.ProtocolGemini,
		BaseURL:  "https://generativelanguage.googleapis.com",
	}

	var capturedReq upstream.Request
	client := &mockUpstreamClient{
		doFunc: func(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error) {
			capturedReq = req
			return upstream.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"candidates": [{
						"content": {"parts": [{"text": "Hi from Gemini"}]}
					}]
				}`),
			}, nil
		},
	}

	service := NewService(client)
	res := service.TestModel(context.Background(), target, "gemini-2.0-flash")

	if !res.OK {
		t.Fatalf("expected OK, got: %s", res.ErrorMessage)
	}
	if capturedReq.Path != "/v1beta/models/gemini-2.0-flash:generateContent" {
		t.Errorf("expected generateContent path, got %s", capturedReq.Path)
	}
	if res.Snippet != "Hi from Gemini" {
		t.Errorf("snippet = %q, want Hi from Gemini", res.Snippet)
	}
}

func TestProbeModelErrorStatus(t *testing.T) {
	target := provider.Target{
		ID:       "antigravity",
		Protocol: provider.ProtocolGemini,
		BaseURL:  "https://cloudcode-pa.googleapis.com",
	}

	client := &mockUpstreamClient{
		doFunc: func(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error) {
			return upstream.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       []byte(`{"error": {"code": 429, "message": "Resource has been exhausted (e.g. check quota).", "status": "RESOURCE_EXHAUSTED"}}`),
			}, nil
		},
	}

	service := NewService(client)
	res := service.TestModel(context.Background(), target, "claude-opus-4-6-thinking")

	if res.OK {
		t.Fatalf("expected failure, got OK")
	}
	if res.StatusCode != 429 {
		t.Errorf("status = %d, want 429", res.StatusCode)
	}
	if !strings.Contains(res.ErrorMessage, "Resource has been exhausted") {
		t.Errorf("error = %q, want Resource has been exhausted", res.ErrorMessage)
	}
}

func TestProbeModelNetworkError(t *testing.T) {
	target := provider.Target{
		ID:       "openai",
		Protocol: provider.ProtocolOpenAI,
		BaseURL:  "https://api.openai.com/v1",
	}

	client := &mockUpstreamClient{
		doFunc: func(ctx context.Context, target provider.Target, req upstream.Request) (upstream.Response, error) {
			return upstream.Response{}, errors.New("connection refused")
		},
	}

	service := NewService(client)
	res := service.TestModel(context.Background(), target, "gpt-4o")

	if res.OK {
		t.Fatalf("expected failure on network error")
	}
	if !strings.Contains(res.ErrorMessage, "connection refused") {
		t.Errorf("error = %q, want connection refused", res.ErrorMessage)
	}
}

func TestProbeEmptyModel(t *testing.T) {
	service := NewService(&mockUpstreamClient{})
	res := service.TestModel(context.Background(), provider.Target{ID: "openai"}, "")
	if res.OK {
		t.Fatalf("expected failure for empty model")
	}
	if res.ErrorMessage != "model ID must not be empty" {
		t.Errorf("unexpected error message: %q", res.ErrorMessage)
	}
}

func TestProbeCustomTimeout(t *testing.T) {
	service := NewService(&mockUpstreamClient{})
	service.SetTimeout(50 * time.Millisecond)
	if service.timeout != 50*time.Millisecond {
		t.Errorf("timeout not updated")
	}
}
