package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerDispatchesAdminHandler(t *testing.T) {
	called := false
	admin := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewWithAdmin(":0", "secret", func() bool { return true }, noop, noop, noop, nil, admin, logger)

	recorder := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/admin", nil))
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, recorder.Code)
	}
}
