package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/usecase/translation"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

func (s *stubCrossTranslator) OpenAIResponsesToAnthropic(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
}

func (s *stubCrossTranslator) OpenAIResponsesToAnthropicStream(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.StreamResponse, error) {
	s.responsesStreamCalled = true
	s.model = model
	return s.responsesStreamResult, s.responsesStreamErr
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

func TestHandlerUsesAnthropicResponsesStreamTranslator(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{responsesStreamResult: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n")),
	}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic},
			Model:  "claude-upstream",
		}},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"portable","stream":true,"input":"hello","max_output_tokens":32}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.responsesStreamCalled || cross.called {
		t.Fatalf("status=%d translator=%#v body=%s", rec.Code, cross, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "response.completed") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "[DONE]") {
		t.Fatalf("Responses stream must not contain [DONE]: %q", rec.Body.String())
	}
}

func TestHandlerFallsBackForUnsupportedResponsesStreamOption(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{responsesStreamErr: apptranslation.CompatibilityError{
		Feature: "stream_options",
		Reason:  "unsupported field",
	}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic}, Model: "claude-upstream"},
			{Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI}, Model: "gpt-upstream"},
		},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"portable","stream":true,"stream_options":{"future":true},"input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.responsesStreamCalled {
		t.Fatalf("status=%d translator=%#v body=%s", rec.Code, cross, rec.Body.String())
	}
	if len(forwarder.calls) != 1 || forwarder.calls[0].providerID != "openai" || forwarder.calls[0].model != "gpt-upstream" {
		t.Fatalf("fallback calls = %#v", forwarder.calls)
	}
}
