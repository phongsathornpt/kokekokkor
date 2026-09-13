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

type geminiResponsesTranslatorStub struct {
	*stubCrossTranslator
	called bool
	model  string
	result upstream.Response
	err    error
}

func (s *geminiResponsesTranslatorStub) OpenAIResponsesToGemini(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
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

func TestHandlerRejectsGeminiResponsesStreamingBeforeTranslator(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &geminiResponsesTranslatorStub{stubCrossTranslator: &stubCrossTranslator{}}
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

	if rec.Code != http.StatusBadRequest || cross.called {
		t.Fatalf("status=%d called=%v body=%s", rec.Code, cross.called, rec.Body.String())
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("raw forwarder calls = %#v", forwarder.calls)
	}
}
