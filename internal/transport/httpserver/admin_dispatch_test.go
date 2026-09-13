package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminRequiresGatewayBearer(t *testing.T) {
	admin := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewWithAdmin(":0", "secret", func() bool { return true }, noop, noop, noop, nil, admin, logger)

	unauthorized := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "http://gateway/admin", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "http://gateway/admin", nil)
	req.Header.Set("Authorization", "Bearer secret")
	authorized := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d", authorized.Code)
	}
}
