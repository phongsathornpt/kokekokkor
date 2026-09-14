package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
)

func TestServerBoundsRequestHeaders(t *testing.T) {
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(":0", "", func() bool { return true }, noop, noop, noop, nil, logger)

	if server.HTTP.MaxHeaderBytes != maxRequestHeaderBytes {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.HTTP.MaxHeaderBytes, maxRequestHeaderBytes)
	}
}
