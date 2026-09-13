package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerDispatchesV1BetaToGemini(t *testing.T) {
	unexpected := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("non-Gemini handler called")
	})
	called := false
	gemini := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := New(":0", "", func() bool { return true }, unexpected, unexpected, gemini, logger)
	req := httptest.NewRequest(http.MethodGet, "http://gateway/v1beta/models", nil)
	rec := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}
