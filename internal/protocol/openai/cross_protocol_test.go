package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type stubCrossTranslator struct {
	called bool
	model  string
	result upstream.Response
	err    error
}

func (s *stubCrossTranslator) OpenAIChatToAnthropic(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
}

func TestHandlerUsesAnthropicTranslator(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{result: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"chatcmpl_gateway"}`),
	}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic},
			Model:  "claude-upstream",
		}},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"portable","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
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

func TestHandlerFallsBackAfterCrossProtocolPreflightRejection(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{err: apptranslation.CompatibilityError{Feature: "response_format", Reason: "unsupported"}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic}, Model: "claude-upstream"},
			{Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI}, Model: "gpt-upstream"},
		},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"portable","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || len(forwarder.calls) != 1 || forwarder.calls[0].providerID != "openai" || forwarder.calls[0].model != "gpt-upstream" {
		t.Fatalf("status=%d calls=%#v body=%s", rec.Code, forwarder.calls, rec.Body.String())
	}
}
