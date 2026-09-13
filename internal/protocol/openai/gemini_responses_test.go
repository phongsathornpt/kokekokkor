package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type geminiResponsesTranslatorStub struct {
	*stubCrossTranslator
	called       bool
	streamCalled bool
	model        string
	result       upstream.Response
	streamResult upstream.StreamResponse
	err          error
	streamErr    error
}

func (s *geminiResponsesTranslatorStub) OpenAIResponsesToGemini(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
}

func (s *geminiResponsesTranslatorStub) OpenAIResponsesToGeminiStream(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.StreamResponse, error) {
	s.streamCalled = true
	s.model = model
	return s.streamResult, s.streamErr
}

func TestHandlerUsesGeminiResponsesTranslator(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &geminiResponsesTranslatorStub{
		stubCrossTranslator: &stubCrossTranslator{},
		result: upstream.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       []byte(`{"id":"resp_gateway","object":"response","status":"completed","output":[]}`),
		},
	}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "gemini", Protocol: provider.ProtocolGemini},
			Model:  "gemini-upstream",
		}},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"portable","input":"hello","max_output_tokens":32}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.called || cross.model != "gemini-upstream" {
		t.Fatalf("status=%d called=%v model=%q body=%s", rec.Code, cross.called, cross.model, rec.Body.String())
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("raw forwarder calls = %#v", forwarder.calls)
	}
}

func TestHandlerUsesGeminiResponsesStreamTranslator(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &geminiResponsesTranslatorStub{
		stubCrossTranslator: &stubCrossTranslator{},
		streamResult: upstream.StreamResponse{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n")),
		},
	}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "gemini", Protocol: provider.ProtocolGemini},
			Model:  "gemini-upstream",
		}},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"portable","stream":true,"input":"hello"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.streamCalled || cross.called || cross.model != "gemini-upstream" {
		t.Fatalf("status=%d streamCalled=%v bufferedCalled=%v model=%q body=%s", rec.Code, cross.streamCalled, cross.called, cross.model, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "response.completed") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("raw forwarder calls = %#v", forwarder.calls)
	}
}
