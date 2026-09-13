package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type testRouter struct{ plan routing.Plan }

func (r testRouter) Resolve(context.Context, routing.Request) (routing.Plan, error) { return r.plan, nil }

type testForwarder struct{ path string }

func (f *testForwarder) ServeHTTPTo(w http.ResponseWriter, r *http.Request, _ provider.Target, _ bool) error {
	f.path = r.URL.Path
	w.WriteHeader(http.StatusOK)
	return nil
}

func TestHandlerRewritesGeminiModelAlias(t *testing.T) {
	forwarder := &testForwarder{}
	handler := NewRoutedHandler(testRouter{plan: routing.Plan{
		RequestedModel: "client-model",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://gemini.example"},
			Model:  "gemini-upstream",
		}},
	}}, nil, forwarder)

	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/client-model:generateContent", strings.NewReader(`{"contents":[]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if forwarder.path != "/v1beta/models/gemini-upstream:generateContent" {
		t.Fatalf("path = %q", forwarder.path)
	}
}
