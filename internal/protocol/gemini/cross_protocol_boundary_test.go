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

type boundaryRouter struct{ plan routing.Plan }

func (r boundaryRouter) Resolve(context.Context, routing.Request) (routing.Plan, error) {
	return r.plan, nil
}

type boundaryForwarder struct{ calls int }

func (f *boundaryForwarder) ServeHTTPTo(w http.ResponseWriter, _ *http.Request, _ provider.Target, _ bool) error {
	f.calls++
	w.WriteHeader(http.StatusOK)
	return nil
}

func TestHandlerRejectsUnsupportedCrossProtocolBeforeUpstream(t *testing.T) {
	forwarder := &boundaryForwarder{}
	handler := NewRoutedHandler(boundaryRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI},
			Model:  "gpt-upstream",
		}},
	}}, nil, forwarder)
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/portable:generateContent", strings.NewReader(`{"contents":[]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if forwarder.calls != 0 {
		t.Fatalf("upstream calls=%d", forwarder.calls)
	}
}
