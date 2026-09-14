package anthropic

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type anthropicStaticBearer struct{ token string }

func (r anthropicStaticBearer) BearerToken(context.Context, string) (string, bool, error) {
	return r.token, r.token != "", nil
}

func TestProxyPrefersOAuthBearerToken(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Api-Key"); got != "" {
			t.Errorf("X-Api-Key = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	proxy := NewWithBearerTokenResolver(slog.New(slog.NewTextHandler(io.Discard, nil)), "2023-06-01", anthropicStaticBearer{token: "oauth-token"})
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1/messages", nil)
	rec := httptest.NewRecorder()
	if err := proxy.ServeHTTPTo(rec, req, provider.Target{ID: "p", BaseURL: upstream.URL, APIKey: "api-key"}, false); err != nil {
		t.Fatalf("ServeHTTPTo() error = %v", err)
	}
}
