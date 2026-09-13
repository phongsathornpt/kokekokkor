package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type routedStub struct {
	plan routing.Plan
	err  error
}

func (s routedStub) Resolve(context.Context, routing.Request) (routing.Plan, error) {
	return s.plan, s.err
}

type stubOpenAITranslator struct {
	called bool
	model  string
	result upstream.Response
	err    error
}

func (s *stubOpenAITranslator) AnthropicMessagesToOpenAI(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
}

func TestRoutedHandlerUsesOpenAITranslator(t *testing.T) {
	forwarder := &fakeForwarder{}
	cross := &stubOpenAITranslator{result: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"type":"message","id":"msg_gateway"}`),
	}}
	handler := NewRoutedHandler(routedStub{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI},
			Model:  "gpt-upstream",
		}},
	}}, nil, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"portable","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.called || cross.model != "gpt-upstream" {
		t.Fatalf("status=%d translator=%#v body=%s", rec.Code, cross, rec.Body.String())
	}
	if forwarder.called {
		t.Fatalf("raw forwarder unexpectedly called: %#v", forwarder)
	}
}
