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

func TestClientAppliesProtocolCredentials(t *testing.T) {
	t.Run("openai", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-Api-Key") != "" {
				t.Errorf("request path=%q authorization=%q x-api-key=%q", r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Api-Key"))
			}
			_, _ = io.WriteString(w, `{"ok":true}`)
		}))
		defer server.Close()

		client := New("2023-06-01")
		_, err := client.Do(context.Background(), provider.Target{ID: "o", Protocol: provider.ProtocolOpenAI, BaseURL: server.URL, APIKey: "secret"}, upstream.Request{
			Method: http.MethodPost,
			Path:   "/v1/chat/completions",
			Body:   []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
	})

	t.Run("anthropic", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "secret" || r.Header.Get("Anthropic-Version") != "2023-06-01" || r.Header.Get("Authorization") != "" {
				t.Errorf("request path=%q x-api-key=%q version=%q authorization=%q", r.URL.Path, r.Header.Get("X-Api-Key"), r.Header.Get("Anthropic-Version"), r.Header.Get("Authorization"))
			}
			_, _ = io.WriteString(w, `{"ok":true}`)
		}))
		defer server.Close()

		client := New("2023-06-01")
		_, err := client.Do(context.Background(), provider.Target{ID: "a", Protocol: provider.ProtocolAnthropic, BaseURL: server.URL, APIKey: "secret"}, upstream.Request{
			Method: http.MethodPost,
			Path:   "/v1/messages",
			Body:   []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
	})
}
