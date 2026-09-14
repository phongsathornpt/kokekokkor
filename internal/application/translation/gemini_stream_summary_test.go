package translation

import (
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestGeminiToResponsesStreamEventAllowsReasoningSummary(t *testing.T) {
	for _, event := range []llm.StreamEvent{
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.ReasoningBlock{}},
		{Type: llm.StreamEventReasoningDelta, Index: 0, ReasoningDelta: "summary"},
		{Type: llm.StreamEventContentStop, Index: 0},
	} {
		if err := GeminiToResponsesStreamEvent(event); err != nil {
			t.Fatalf("event %q rejected: %v", event.Type, err)
		}
	}
}

func TestGeminiToOpenAIStreamEventStillRejectsReasoning(t *testing.T) {
	err := GeminiToOpenAIStreamEvent(llm.StreamEvent{Type: llm.StreamEventReasoningDelta, Index: 0, ReasoningDelta: "summary"})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestGeminiToResponsesStreamEventRejectsProviderReasoningState(t *testing.T) {
	err := GeminiToResponsesStreamEvent(llm.StreamEvent{
		Type:  llm.StreamEventContentStart,
		Index: 0,
		Block: llm.ReasoningBlock{Signature: "opaque"},
	})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}
