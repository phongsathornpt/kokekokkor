package anthropic

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
)

func TestProxyBoundsResponseHeaders(t *testing.T) {
	proxy := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "2023-06-01")
	transport, ok := proxy.transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", proxy.transport)
	}
	if transport.MaxResponseHeaderBytes != maxResponseHeaderBytes {
		t.Fatalf("MaxResponseHeaderBytes = %d, want %d", transport.MaxResponseHeaderBytes, maxResponseHeaderBytes)
	}
}
