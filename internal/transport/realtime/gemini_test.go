package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	ws "github.com/phongsathornpt/kokekokkor/internal/transport/websocket"
)

type staticBearer struct{ token string }

func (s staticBearer) BearerToken(context.Context, string) (string, bool, error) {
	return s.token, s.token != "", nil
}

func TestGeminiBridgeRealtimeTextRoundTrip(t *testing.T) {
	upstreamDone := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != geminiLivePath {
			t.Errorf("upstream path = %q", r.URL.Path)
		}
		if r.Header.Get("X-Goog-Api-Key") != "gemini-key" {
			t.Errorf("X-Goog-Api-Key = %q", r.Header.Get("X-Goog-Api-Key"))
		}
		conn, err := ws.Accept(w, r)
		if err != nil {
			upstreamDone <- err
			return
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		setup, err := conn.ReadText(ctx)
		if err != nil {
			upstreamDone <- err
			return
		}
		if !strings.Contains(string(setup), `"model":"models/gemini-live-target"`) {
			t.Errorf("setup = %s", setup)
		}
		if err := conn.WriteText(ctx, []byte(`{"setupComplete":{}}`)); err != nil {
			upstreamDone <- err
			return
		}

		content, err := conn.ReadText(ctx)
		if err != nil {
			upstreamDone <- err
			return
		}
		if !strings.Contains(string(content), `"text":"hello"`) {
			t.Errorf("client content = %s", content)
		}
		turnComplete, err := conn.ReadText(ctx)
		if err != nil {
			upstreamDone <- err
			return
		}
		if string(turnComplete) != `{"clientContent":{"turnComplete":true}}` {
			t.Errorf("turnComplete = %s", turnComplete)
		}

		if err := conn.WriteText(ctx, []byte(`{"serverContent":{"modelTurn":{"parts":[{"text":"hi there"}]}}}`)); err != nil {
			upstreamDone <- err
			return
		}
		if err := conn.WriteText(ctx, []byte(`{"serverContent":{"turnComplete":true},"usageMetadata":{"promptTokenCount":3,"responseTokenCount":2}}`)); err != nil {
			upstreamDone <- err
			return
		}
		upstreamDone <- nil
	}))
	defer upstream.Close()

	bridge := NewGeminiBridge(nil)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = bridge.ServeOpenAIRealtimeToGemini(w, r, provider.Target{
			ID:       "gemini",
			Protocol: provider.ProtocolGemini,
			BaseURL:  upstream.URL,
			APIKey:   "gemini-key",
		}, "portable-model", "gemini-live-target")
	}))
	defer gateway.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	clientURL := "ws" + strings.TrimPrefix(gateway.URL, "http") + "/v1/realtime?model=portable-model"
	client, _, err := ws.Dial(ctx, clientURL, nil)
	if err != nil {
		t.Fatalf("dial gateway: %v", err)
	}
	defer client.Close()

	created, err := client.ReadText(ctx)
	if err != nil {
		t.Fatalf("read session.created: %v", err)
	}
	if !strings.Contains(string(created), `"type":"session.created"`) || !strings.Contains(string(created), `"model":"portable-model"`) {
		t.Fatalf("session.created = %s", created)
	}

	for _, payload := range []string{
		`{"type":"session.update","session":{"output_modalities":["text"]}}`,
		`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`,
		`{"type":"response.create"}`,
	} {
		if err := client.WriteText(ctx, []byte(payload)); err != nil {
			t.Fatalf("write client event: %v", err)
		}
	}

	var sawDelta, sawDone bool
	for !sawDone {
		payload, err := client.ReadText(ctx)
		if err != nil {
			t.Fatalf("read OpenAI server event: %v", err)
		}
		var event struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response struct {
				Status string `json:"status"`
			} `json:"response"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode OpenAI event %s: %v", payload, err)
		}
		if event.Type == "response.output_text.delta" && event.Delta == "hi there" {
			sawDelta = true
		}
		if event.Type == "response.done" {
			sawDone = event.Response.Status == "completed"
		}
	}
	if !sawDelta {
		t.Fatal("did not receive translated text delta")
	}
	if err := <-upstreamDone; err != nil {
		t.Fatalf("fake Gemini upstream: %v", err)
	}
}

func TestGeminiBridgeOAuthTakesPrecedence(t *testing.T) {
	bridge := NewGeminiBridge(staticBearer{token: "oauth-token"})
	url, header, err := bridge.upstreamHandshake(context.Background(), provider.Target{
		ID:       "gemini",
		Protocol: provider.ProtocolGemini,
		BaseURL:  "https://generativelanguage.googleapis.com",
		APIKey:   "api-key",
	})
	if err != nil {
		t.Fatalf("upstreamHandshake() error = %v", err)
	}
	if !strings.HasPrefix(url, "wss://generativelanguage.googleapis.com/") {
		t.Fatalf("url = %q", url)
	}
	if header.Get("Authorization") != "Bearer oauth-token" {
		t.Fatalf("Authorization = %q", header.Get("Authorization"))
	}
	if header.Get("X-Goog-Api-Key") != "" {
		t.Fatalf("X-Goog-Api-Key = %q", header.Get("X-Goog-Api-Key"))
	}
}
