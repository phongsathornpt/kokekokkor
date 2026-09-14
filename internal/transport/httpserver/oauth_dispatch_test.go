package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRedirectsOAuthStartIntoAdminSurface(t *testing.T) {
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	oauth := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	admin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/oauth/gemini/start" {
			t.Fatalf("admin path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewWithAdmin(":0", "secret", func() bool { return true }, noop, noop, noop, oauth, admin, logger)

	start := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "http://gateway/oauth/gemini/start", nil))
	if start.Code != http.StatusSeeOther || start.Header().Get("Location") != "/admin/oauth/gemini/start" {
		t.Fatalf("start status=%d location=%q", start.Code, start.Header().Get("Location"))
	}

	protected := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "http://gateway/admin/oauth/gemini/start", nil))
	if protected.Code != http.StatusAccepted {
		t.Fatalf("protected status=%d", protected.Code)
	}

	callback := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(callback, httptest.NewRequest(http.MethodGet, "http://gateway/oauth/gemini/callback", nil))
	if callback.Code != http.StatusTeapot {
		t.Fatalf("callback status=%d", callback.Code)
	}
}

func TestServerDoesNotExposeOAuthStartWithoutAdmin(t *testing.T) {
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	oauth := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := NewWithAdmin(":0", "secret", func() bool { return true }, noop, noop, noop, oauth, nil, logger)

	start := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "http://gateway/oauth/gemini/start", nil))
	if start.Code != http.StatusNotFound {
		t.Fatalf("start status=%d, want 404", start.Code)
	}

	callback := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(callback, httptest.NewRequest(http.MethodGet, "http://gateway/oauth/gemini/callback", nil))
	if callback.Code != http.StatusTeapot {
		t.Fatalf("callback status=%d", callback.Code)
	}
}
