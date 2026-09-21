package sdk_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/pkg/sdk"
)

func TestNewClient(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		client, err := sdk.New("http://localhost:8080/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.BaseURL() != "http://localhost:8080" {
			t.Errorf("expected http://localhost:8080, got %s", client.BaseURL())
		}
	})

	t.Run("invalid URL without scheme or host", func(t *testing.T) {
		_, err := sdk.New("localhost:8080")
		if err == nil {
			t.Fatal("expected error for URL without scheme, got nil")
		}
	})

	t.Run("with options", func(t *testing.T) {
		customHTTP := &http.Client{Timeout: 5 * time.Second}
		client, err := sdk.New("http://localhost:8080",
			sdk.WithAPIKey("secret-key"),
			sdk.WithHTTPClient(customHTTP),
			sdk.WithTimeout(2*time.Second),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})
}

func TestHealthEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := sdk.New(server.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	live, err := client.Live(ctx)
	if err != nil {
		t.Fatalf("live check failed: %v", err)
	}
	if live.Status != "ok" {
		t.Errorf("expected status ok, got %s", live.Status)
	}

	ready, err := client.Ready(ctx)
	if err != nil {
		t.Fatalf("ready check failed: %v", err)
	}
	if ready.Status != "ready" {
		t.Errorf("expected status ready, got %s", ready.Status)
	}
}

func TestChatCompletion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`invalid api key`))
			return
		}

		var req sdk.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Model != "gpt-4o" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`unknown model`))
			return
		}

		resp := sdk.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1677652288,
			Model:   "gpt-4o",
			Choices: []sdk.ChatChoice{
				{
					Index: 0,
					Message: sdk.ChatMessage{
						Role:    sdk.RoleAssistant,
						Content: "Hello! How can I assist you today?",
					},
					FinishReason: "stop",
				},
			},
			Usage: sdk.Usage{
				PromptTokens:     9,
				CompletionTokens: 12,
				TotalTokens:      21,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	ctx := context.Background()

	t.Run("successful completion", func(t *testing.T) {
		client, err := sdk.New(server.URL, sdk.WithAPIKey("test-api-key"))
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		resp, err := client.ChatCompletion(ctx, &sdk.ChatCompletionRequest{
			Model: "gpt-4o",
			Messages: []sdk.ChatMessage{
				{Role: sdk.RoleUser, Content: "Hello!"},
			},
		})
		if err != nil {
			t.Fatalf("chat completion failed: %v", err)
		}
		if resp.ID != "chatcmpl-123" {
			t.Errorf("expected id chatcmpl-123, got %s", resp.ID)
		}
		if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "Hello! How can I assist you today?" {
			t.Errorf("unexpected choices: %+v", resp.Choices)
		}
	})

	t.Run("unauthorized error", func(t *testing.T) {
		client, err := sdk.New(server.URL, sdk.WithAPIKey("wrong-key"))
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		_, err = client.ChatCompletion(ctx, &sdk.ChatCompletionRequest{
			Model: "gpt-4o",
			Messages: []sdk.ChatMessage{
				{Role: sdk.RoleUser, Content: "Hello!"},
			},
		})
		if !errors.Is(err, sdk.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("nil request error", func(t *testing.T) {
		client, err := sdk.New(server.URL)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		_, err = client.ChatCompletion(ctx, nil)
		if !errors.Is(err, sdk.ErrBadRequest) {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
	})
}

func TestMessages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "anthropic-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`missing or invalid x-api-key`))
			return
		}
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp := sdk.MessagesResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []sdk.ContentBlock{
				{Type: "text", Text: "I am Claude."},
			},
			Model:      "claude-3-5-sonnet-20241022",
			StopReason: "end_turn",
			Usage: sdk.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	ctx := context.Background()

	t.Run("successful messages request", func(t *testing.T) {
		client, err := sdk.New(server.URL, sdk.WithAPIKey("anthropic-key"))
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		resp, err := client.Messages(ctx, &sdk.MessagesRequest{
			Model: "claude-3-5-sonnet-20241022",
			Messages: []sdk.AnthropicMessage{
				{
					Role: "user",
					Content: []sdk.ContentBlock{
						{Type: "text", Text: "Who are you?"},
					},
				},
			},
			MaxTokens: 100,
		})
		if err != nil {
			t.Fatalf("messages call failed: %v", err)
		}
		if resp.ID != "msg_123" {
			t.Errorf("expected id msg_123, got %s", resp.ID)
		}
		if len(resp.Content) == 0 || resp.Content[0].Text != "I am Claude." {
			t.Errorf("unexpected content: %+v", resp.Content)
		}
	})

	t.Run("nil request returns ErrBadRequest", func(t *testing.T) {
		client, err := sdk.New(server.URL)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		_, err = client.Messages(ctx, nil)
		if !errors.Is(err, sdk.ErrBadRequest) {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
	})
}
