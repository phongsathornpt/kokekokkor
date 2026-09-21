package proxy_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/handler/proxy"
)

func TestServerDispatchesAdminHandler(t *testing.T) {
	called := false
	admin := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := proxy.NewWithAdmin(":0", "secret", func() bool { return true }, noop, noop, noop, nil, admin, logger)

	recorder := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://gateway/admin", nil))
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, recorder.Code)
	}
}

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
	server := proxy.New(":0", "", func() bool { return true }, unexpected, unexpected, gemini, nil, logger)
	req := httptest.NewRequest(http.MethodGet, "http://gateway/v1beta/models", nil)
	rec := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, rec.Code)
	}
}

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
	server := proxy.NewWithAdmin(":0", "secret", func() bool { return true }, noop, noop, noop, oauth, admin, logger)

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
	server := proxy.New(":0", "", func() bool { return true }, noop, noop, noop, oauth, logger)

	rec := httptest.NewRecorder()
	server.HTTP.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://gateway/oauth/gemini/start", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}

func TestServerBoundsRequestHeaders(t *testing.T) {
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := proxy.New(":0", "", func() bool { return true }, noop, noop, noop, nil, logger)

	if server.HTTP.MaxHeaderBytes != 64<<10 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", server.HTTP.MaxHeaderBytes, 64<<10)
	}
}

func TestServerHealthEndpoints(t *testing.T) {
	noop := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	ready := true
	server := proxy.New(":0", "", func() bool { return ready }, noop, noop, noop, nil, nil)

	t.Run("health live returns 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		server.HTTP.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://gateway/health/live", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("live status=%d", rec.Code)
		}
	})

	t.Run("health ready returns 200 when ready", func(t *testing.T) {
		rec := httptest.NewRecorder()
		server.HTTP.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://gateway/health/ready", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("ready status=%d", rec.Code)
		}
	})

	t.Run("health ready returns 503 when not ready", func(t *testing.T) {
		ready = false
		rec := httptest.NewRecorder()
		server.HTTP.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://gateway/health/ready", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("not ready status=%d", rec.Code)
		}
	})
}
