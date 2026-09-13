package anthropic

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestProxyReplacesClientCredentialsAndPreservesAnthropicHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/messages" {
			t.Errorf("path = %q", got)
		}
		if got := r.Header.Get("X-Api-Key"); got != "upstream-secret" {
			t.Errorf("X-Api-Key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization leaked upstream: %q", got)
		}
		if got := r.Header.Get("Anthropic-Version"); got != "2025-01-01" {
			t.Errorf("Anthropic-Version = %q", got)
		}
		if got := r.Header.Get("Anthropic-Beta"); got != "prompt-caching-2024-07-31" {
			t.Errorf("Anthropic-Beta = %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "2023-06-01")
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	req.Header.Set("X-Api-Key", "gateway-client-key")
	req.Header.Set("Authorization", "Bearer gateway-client-key")
	req.Header.Set("Anthropic-Version", "2025-01-01")
	req.Header.Set("Anthropic-Beta", "prompt-caching-2024-07-31")
	rec := httptest.NewRecorder()

	err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "anthropic", BaseURL: upstream.URL, APIKey: "upstream-secret"}, false)
	if err != nil {
		t.Fatalf("ServeHTTPTo() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "message_stop") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestProxyAddsDefaultAnthropicVersion(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Anthropic-Version"); got != "2023-06-01" {
			t.Errorf("Anthropic-Version = %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "2023-06-01")
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1/messages/count_tokens", strings.NewReader(`{"model":"claude-test"}`))
	rec := httptest.NewRecorder()
	if err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "anthropic", BaseURL: upstream.URL}, false); err != nil {
		t.Fatalf("ServeHTTPTo() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestProxySuppressesRetryableStatusBeforeFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "unavailable")
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "2023-06-01")
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	rec := httptest.NewRecorder()

	err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "anthropic", BaseURL: upstream.URL}, true)
	if err == nil {
		t.Fatal("ServeHTTPTo() error = nil, want retryable error")
	}
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("response committed during fallback: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
