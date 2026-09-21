package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/usecase/translation"
)

func TestHandlerDoesNotFallbackAfterResponseTranslationFailure(t *testing.T) {
	forwarder := &stubForwarder{fail: map[string]error{}}
	cross := &stubCrossTranslator{err: apptranslation.ResponseError{Err: apptranslation.CompatibilityError{
		Feature: "thinking",
		Reason:  "cannot represent response",
	}}}
	handler := NewHandler(stubRouter{plan: routing.Plan{
		RequestedModel: "portable",
		Attempts: []routing.Attempt{
			{Target: provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic}, Model: "claude"},
			{Target: provider.Target{ID: "backup", Protocol: provider.ProtocolOpenAI}, Model: "gpt"},
		},
	}}, forwarder, cross)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"portable","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rec.Code, rec.Body.String())
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("fallback executed after completed generation: %#v", forwarder.calls)
	}
}
