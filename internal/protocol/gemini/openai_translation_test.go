package gemini

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

type openAITranslatorStub struct {
	called bool
	model  string
	body   string
}

func (s *openAITranslatorStub) GeminiGenerateContentToOpenAI(_ context.Context, _ provider.Target, model string, _ http.Header, body []byte) (upstream.Response, error) {
	s.called = true
	s.model = model
	s.body = string(body)
	return upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}]}`),
	}, nil
}

func TestHandlerTranslatesGenerateContentToOpenAITarget(t *testing.T) {
	translator := &openAITranslatorStub{}
	handler := NewRoutedHandler(testRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{{
			Target: provider.Target{ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example"},
			Model:  "gpt-upstream",
		}},
	}}, nil, &testForwarder{}, translator)

	body := `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "http://gateway/v1beta/models/portable:generateContent", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if !translator.called || translator.model != "gpt-upstream" || translator.body != body {
		t.Fatalf("translator called=%v model=%q body=%q", translator.called, translator.model, translator.body)
	}
}
