package openai

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

func (s *stubCrossTranslator) OpenAIResponsesToAnthropic(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
}

func TestHandlerUsesAnthropicResponsesTranslator(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{result: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"resp_gateway","object":"response","status":"completed","output":[]}`),
	}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic},
			Model:  "claude-upstream",
		}},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"portable","input":"hello","max_output_tokens":32}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.called || cross.model != "claude-upstream" {
		t.Fatalf("status=%d translator=%#v body=%s", rec.Code, cross, rec.Body.String())
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("raw forwarder calls = %#v", forwarder.calls)
	}
}

func TestHandlerFallsBackForResponsesStreaming(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic}, Model: "claude-upstream"},
			{Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI}, Model: "gpt-upstream"},
		},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"portable","stream":true,"input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || cross.called {
		t.Fatalf("status=%d translator=%#v body=%s", rec.Code, cross, rec.Body.String())
	}
	if len(forwarder.calls) != 1 || forwarder.calls[0].providerID != "openai" || forwarder.calls[0].model != "gpt-upstream" {
		t.Fatalf("fallback calls = %#v", forwarder.calls)
	}
}
