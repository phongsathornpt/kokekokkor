package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDialAndAcceptRoundTrip(t *testing.T) {
	handlerDone := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Accept(w, r)
		if err != nil {
			handlerDone <- err
			return
		}
		defer conn.Close()
		message, err := conn.ReadText(r.Context())
		if err == nil {
			err = conn.WriteText(r.Context(), append([]byte("echo:"), message...))
		}
		handlerDone <- err
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, response, err := Dial(ctx, wsURL, http.Header{"X-Test": []string{"value"}})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if err := conn.WriteText(ctx, []byte("hello")); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	message, err := conn.ReadText(ctx)
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	if string(message) != "echo:hello" {
		t.Fatalf("message = %q", message)
	}
	if err := <-handlerDone; err != nil {
		t.Fatalf("handler error = %v", err)
	}
}

func TestWebsocketAcceptRFCExample(t *testing.T) {
	const key = "dGhlIHNhbXBsZSBub25jZQ=="
	const want = "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got := websocketAccept(key); got != want {
		t.Fatalf("websocketAccept() = %q, want %q", got, want)
	}
}

func TestHeaderContainsToken(t *testing.T) {
	header := http.Header{"Connection": []string{"keep-alive, Upgrade"}}
	if !headerContainsToken(header, "Connection", "upgrade") {
		t.Fatal("upgrade token not found")
	}
}
