package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicAPIKeyAuthAcceptsXAPIKey(t *testing.T) {
	called := false
	handler := anthropicAPIKeyAuth("gateway-secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("X-Api-Key", "gateway-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}

func TestAnthropicAPIKeyAuthReturnsAnthropicError(t *testing.T) {
	handler := anthropicAPIKeyAuth("gateway-secret", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("X-Api-Key", "wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"type":"error"`) || !strings.Contains(body, `"authentication_error"`) {
		t.Fatalf("body=%q", body)
	}
}
