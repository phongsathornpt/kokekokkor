package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeminiAPIKeyAuthAcceptsQueryKey(t *testing.T) {
	called := false
	handler := geminiAPIKeyAuth("gateway-secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/gemini-test:streamGenerateContent?alt=sse&key=gateway-secret", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}
