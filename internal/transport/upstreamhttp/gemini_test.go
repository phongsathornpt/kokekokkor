package upstreamhttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

func TestClientAppliesGeminiCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("X-Goog-Api-Key"); got != "secret" {
			t.Errorf("X-Goog-Api-Key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization leaked upstream: %q", got)
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := New("2023-06-01")
	_, err := client.Do(context.Background(), provider.Target{
		ID: "g", Protocol: provider.ProtocolGemini, BaseURL: server.URL, APIKey: "secret",
	}, upstream.Request{
		Method: http.MethodPost,
		Path:   "/v1beta/models/gemini-test:generateContent",
		Header: http.Header{"Authorization": []string{"Bearer client-secret"}},
		Body:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}
