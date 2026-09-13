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

	proxy.ServeHTTPTo(rec, req, provider.Target{ID: "test", BaseURL: upstream.URL, APIKey: "upstream-secret"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Body.String(); got != `{"ok":true}` {
		t.Fatalf("body = %q", got)
	}
}
