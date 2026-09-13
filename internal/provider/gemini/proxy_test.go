package gemini

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestProxyReplacesClientCredentialsAndPreservesGeminiQuery(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1beta/models/gemini-test:streamGenerateContent" {
			t.Errorf("path = %q", got)
		}
		if got := r.URL.Query().Get("alt"); got != "sse" {
			t.Errorf("alt = %q", got)
		}
		if got := r.URL.Query().Get("key"); got != "" {
			t.Errorf("client key leaked in query: %q", got)
		}
		if got := r.Header.Get("X-Goog-Api-Key"); got != "upstream-secret" {
			t.Errorf("X-Goog-Api-Key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization leaked upstream: %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: {\"candidates\":[]}\n\n")
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/gemini-test:streamGenerateContent?alt=sse&key=gateway-secret", strings.NewReader(`{"contents":[]}`))
	req.Header.Set("X-Goog-Api-Key", "gateway-secret")
	req.Header.Set("Authorization", "Bearer gateway-secret")
	rec := httptest.NewRecorder()

	err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: upstream.URL, APIKey: "upstream-secret"}, false)
	if err != nil {
		t.Fatalf("ServeHTTPTo() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "candidates") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestProxySuppressesRetryableStatusBeforeFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "unavailable")
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/gemini-test:generateContent", strings.NewReader(`{"contents":[]}`))
	rec := httptest.NewRecorder()

	err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: upstream.URL}, true)
	if err == nil {
		t.Fatal("ServeHTTPTo() error = nil, want retryable error")
	}
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("response committed during fallback: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
