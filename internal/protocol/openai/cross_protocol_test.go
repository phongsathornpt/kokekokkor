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

type stubCrossTranslator struct {
	called                bool
	streamCalled          bool
	responsesStreamCalled bool
	model                 string
	result                upstream.Response
	streamResult          upstream.StreamResponse
	responsesStreamResult upstream.StreamResponse
	err                   error
	streamErr             error
	responsesStreamErr    error
}

func (s *stubCrossTranslator) OpenAIChatToAnthropic(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
}

func (s *stubCrossTranslator) OpenAIChatToAnthropicStream(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.StreamResponse, error) {
	s.streamCalled = true
	s.model = model
	return s.streamResult, s.streamErr
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

func TestHandlerUsesAnthropicStreamTranslator(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{streamResult: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"ok\":true}\n\ndata: [DONE]\n\n")),
	}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic},
			Model:  "claude-upstream",
		}},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"portable","stream":true,"max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.streamCalled || cross.called {
		t.Fatalf("status=%d translator=%#v body=%s", rec.Code, cross, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Fatalf("body = %q", rec.Body.String())
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
