package anthropic

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

type routedStub struct {
	plan routing.Plan
	err  error
}

func (s routedStub) Resolve(context.Context, routing.Request) (routing.Plan, error) {
	return s.plan, s.err
}

type stubOpenAITranslator struct {
	called       bool
	streamCalled bool
	model        string
	result       upstream.Response
	streamResult upstream.StreamResponse
	err          error
	streamErr    error
}

func (s *stubOpenAITranslator) AnthropicMessagesToOpenAI(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	return s.result, s.err
}

func (s *stubOpenAITranslator) AnthropicMessagesToOpenAIStream(_ context.Context, _ provider.Target, model string, _ http.Header, _ []byte) (upstream.StreamResponse, error) {
	s.streamCalled = true
	s.model = model
	return s.streamResult, s.streamErr
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

func TestRoutedHandlerUsesOpenAIStreamTranslator(t *testing.T) {
	forwarder := &fakeForwarder{}
	cross := &stubOpenAITranslator{streamResult: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")),
	}}
	handler := NewRoutedHandler(routedStub{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI},
			Model:  "gpt-upstream",
		}},
	}}, nil, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"portable","stream":true,"max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !cross.streamCalled || cross.called {
		t.Fatalf("status=%d translator=%#v body=%s", rec.Code, cross, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "message_stop") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}
