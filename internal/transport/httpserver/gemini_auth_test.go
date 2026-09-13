package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiAPIKeyAuthAcceptsHeader(t *testing.T) {
	called := false
	handler := geminiAPIKeyAuth("gateway-secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/gemini-test:generateContent", nil)
	req.Header.Set("X-Goog-Api-Key", "gateway-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}

func TestGeminiAPIKeyAuthReturnsGoogleError(t *testing.T) {
	handler := geminiAPIKeyAuth("gateway-secret", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler called")
	}))
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/gemini-test:generateContent", nil)
	req.Header.Set("X-Goog-Api-Key", "wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"status":"UNAUTHENTICATED"`) {
		t.Fatalf("body = %q", body)
	}
}
