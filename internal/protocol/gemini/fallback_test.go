package gemini

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type fallbackRouter struct{ plan routing.Plan }

func (r fallbackRouter) Resolve(context.Context, routing.Request) (routing.Plan, error) {
	return r.plan, nil
}

type fallbackForwarder struct {
	calls  int
	bodies []string
}

func (f *fallbackForwarder) ServeHTTPTo(w http.ResponseWriter, r *http.Request, _ provider.Target, allowFallback bool) error {
	f.calls++
	body, _ := io.ReadAll(r.Body)
	f.bodies = append(f.bodies, string(body))
	if allowFallback {
		return errors.New("retry")
	}
	w.WriteHeader(http.StatusOK)
	return nil
}

func TestHandlerReplaysBodyAcrossGeminiFallback(t *testing.T) {
	forwarder := &fallbackForwarder{}
	handler := NewRoutedHandler(fallbackRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "a", Protocol: provider.ProtocolGemini}, Model: "gemini-a"},
			{Target: provider.Target{ID: "b", Protocol: provider.ProtocolGemini}, Model: "gemini-b"},
		},
	}}, nil, forwarder)
	body := `{"contents":[]}`
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/portable:generateContent", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || forwarder.calls != 2 {
		t.Fatalf("status=%d calls=%d", rec.Code, forwarder.calls)
	}
	if len(forwarder.bodies) != 2 || forwarder.bodies[0] != body || forwarder.bodies[1] != body {
		t.Fatalf("bodies=%#v", forwarder.bodies)
	}
}
