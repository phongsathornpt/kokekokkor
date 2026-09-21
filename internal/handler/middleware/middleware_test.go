package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/handler/middleware"
)

func TestBearerAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := middleware.BearerAuth("secret", next)

	t.Run("accepts configured key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("rejects invalid key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("passes through when expected key is empty", func(t *testing.T) {
		openHandler := middleware.BearerAuth("", next)
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()
		openHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestAnthropicAPIKeyAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := middleware.AnthropicAPIKeyAuth("anthropic-secret", next)

	t.Run("accepts X-Api-Key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		req.Header.Set("X-Api-Key", "anthropic-secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("accepts Bearer token fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer anthropic-secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("rejects invalid key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		req.Header.Set("X-Api-Key", "wrong-secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestGeminiAPIKeyAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := middleware.GeminiAPIKeyAuth("gemini-secret", next)

	t.Run("accepts X-Goog-Api-Key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1beta/models", nil)
		req.Header.Set("X-Goog-Api-Key", "gemini-secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("accepts query param key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1beta/models?key=gemini-secret", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("accepts Bearer token fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1beta/models", nil)
		req.Header.Set("Authorization", "Bearer gemini-secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("rejects invalid key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1beta/models", nil)
		req.Header.Set("X-Goog-Api-Key", "wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestRequestID(t *testing.T) {
	var downstream string
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstream = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("preserves client request ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("X-Request-ID", "client-id-123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if downstream != "client-id-123" {
			t.Fatalf("downstream request ID = %q", downstream)
		}
		if got := rec.Header().Get("X-Request-ID"); got != "client-id-123" {
			t.Fatalf("response request ID = %q", got)
		}
	})

	t.Run("generates ID when missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if downstream == "" {
			t.Fatal("expected generated request ID, got empty")
		}
		if got := rec.Header().Get("X-Request-ID"); got != downstream {
			t.Fatalf("response request ID = %q, downstream = %q", got, downstream)
		}
	})

	t.Run("replaces oversized ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("X-Request-ID", strings.Repeat("x", 129))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if downstream == "" || len(downstream) > 128 {
			t.Fatalf("invalid replaced request ID = %q", downstream)
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.SecurityHeaders(next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", got)
	}
}

func TestMaxBytes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	handler := middleware.MaxBytes(10, next)

	t.Run("accepts body under limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("short"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("rejects body over limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("this payload exceeds 10 bytes"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", rec.Code)
		}
	})
}
