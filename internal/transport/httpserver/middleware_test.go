package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := bearerAuth("secret", next)

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
}

func TestRequestIDPreservesClientIDAndPropagatesDownstream(t *testing.T) {
	var downstream string
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstream = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("X-Request-ID", "client-request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if downstream != "client-request" {
		t.Fatalf("downstream request ID = %q", downstream)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "client-request" {
		t.Fatalf("response request ID = %q", got)
	}
}

func TestRequestIDGeneratesAndPropagatesID(t *testing.T) {
	var downstream string
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstream = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if downstream == "" {
		t.Fatal("downstream request ID is empty")
	}
	if got := rec.Header().Get("X-Request-ID"); got != downstream {
		t.Fatalf("response request ID = %q, downstream = %q", got, downstream)
	}
}

func TestRequestIDReplacesOversizedID(t *testing.T) {
	var downstream string
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstream = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("X-Request-ID", string(make([]byte, 129)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if downstream == "" || len(downstream) > 128 {
		t.Fatalf("replacement request ID = %q", downstream)
	}
}
