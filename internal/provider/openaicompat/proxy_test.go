package openaicompat

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestProxyForwardsRequestAndReplacesAuthorization(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/chat/completions" {
			t.Errorf("path = %q", got)
		}
		if got := r.URL.RawQuery; got != "beta=true" {
			t.Errorf("query = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer upstream-secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Kokekokkor-Provider"); got != "" {
			t.Errorf("internal routing header leaked upstream: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1/chat/completions?beta=true", strings.NewReader(`{"model":"x"}`))
	req.Header.Set("Authorization", "Bearer client-secret")
	req.Header.Set("X-Kokekokkor-Provider", "should-not-leak")
	rec := httptest.NewRecorder()

	if err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "test", BaseURL: upstream.URL, APIKey: "upstream-secret"}, false); err != nil {
		t.Fatalf("ServeHTTPTo() error = %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Body.String(); got != `{"ok":true}` {
		t.Fatalf("body = %q", got)
	}
}

func TestProxyDefersRetryableStatusWhenFallbackAvailable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Upstream", "failed")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "try later")
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	target := provider.Target{ID: "primary", BaseURL: upstream.URL}

	retryReq := httptest.NewRequest(http.MethodPost, "http://gateway/v1/responses", strings.NewReader(`{"model":"x"}`))
	retryRec := httptest.NewRecorder()
	if err := proxy.ServeHTTPTo(retryRec, retryReq, target, true); err == nil {
		t.Fatal("ServeHTTPTo() error = nil, want retryable error")
	}
	if retryRec.Body.Len() != 0 {
		t.Fatalf("retryable response leaked body %q", retryRec.Body.String())
	}
	if got := retryRec.Header().Get("X-Upstream"); got != "" {
		t.Fatalf("retryable response leaked header %q", got)
	}

	finalReq := httptest.NewRequest(http.MethodPost, "http://gateway/v1/responses", strings.NewReader(`{"model":"x"}`))
	finalRec := httptest.NewRecorder()
	if err := proxy.ServeHTTPTo(finalRec, finalReq, target, false); err != nil {
		t.Fatalf("final ServeHTTPTo() error = %v", err)
	}
	if finalRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("final status = %d, want %d", finalRec.Code, http.StatusServiceUnavailable)
	}
	if finalRec.Body.String() != "try later" {
		t.Fatalf("final body = %q", finalRec.Body.String())
	}
}

func TestProxyDoesNotFallbackOnClientError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "bad request")
	}))
	defer upstream.Close()

	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1/responses", strings.NewReader(`{"model":"x"}`))
	rec := httptest.NewRecorder()

	if err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "primary", BaseURL: upstream.URL}, true); err != nil {
		t.Fatalf("ServeHTTPTo() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
