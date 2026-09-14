package translation

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestAnthropicToResponsesStreamEventAllowsPortableError(t *testing.T) {
	event := llm.StreamEvent{
		Type: llm.StreamEventError,
		Error: &llm.StreamError{Code: "overloaded_error", Message: "busy", Retryable: true},
	}
	if err := AnthropicToResponsesStreamEvent(event); err != nil {
		t.Fatalf("AnthropicToResponsesStreamEvent() error = %v", err)
	}
}

func TestAnthropicToResponsesStreamEventRejectsMissingErrorPayload(t *testing.T) {
	if err := AnthropicToResponsesStreamEvent(llm.StreamEvent{Type: llm.StreamEventError}); err == nil {
		t.Fatal("AnthropicToResponsesStreamEvent() error = nil")
	}
}
