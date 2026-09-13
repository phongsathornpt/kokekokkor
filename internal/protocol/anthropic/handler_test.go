package anthropic

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type fakeForwarder struct {
	called bool
	target provider.Target
}

func (f *fakeForwarder) ServeHTTPTo(w http.ResponseWriter, _ *http.Request, target provider.Target) {
	f.called = true
	f.target = target
	w.WriteHeader(http.StatusAccepted)
}

func TestHandlerForwardsToConfiguredTarget(t *testing.T) {
	target := &provider.Target{ID: "anthropic", BaseURL: "https://api.anthropic.com", APIKey: "secret"}
	forwarder := &fakeForwarder{}
	handler := NewHandler(target, forwarder)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !handler.Ready() {
		t.Fatal("Ready() = false, want true")
	}
	if !forwarder.called || forwarder.target.ID != "anthropic" {
		t.Fatalf("forwarder = %#v", forwarder)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestHandlerWithoutTargetReturnsAnthropicError(t *testing.T) {
	forwarder := &fakeForwarder{}
	handler := NewHandler(nil, forwarder)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if handler.Ready() {
		t.Fatal("Ready() = true, want false")
	}
	if forwarder.called {
		t.Fatal("forwarder called without target")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); got == "" || got[0] != '{' {
		t.Fatalf("unexpected body %q", got)
	}
}
