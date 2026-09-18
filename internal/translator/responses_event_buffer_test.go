package translator

import (
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestResponsesEventBufferBoundsBytes(t *testing.T) {
	var buffer responsesEventBuffer
	if err := buffer.Append(llm.StreamEvent{Type: llm.StreamEventTextDelta, TextDelta: "ok"}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	err := buffer.Append(llm.StreamEvent{
		Type:      llm.StreamEventTextDelta,
		TextDelta: strings.Repeat("x", maxBufferedResponsesBytes),
	})
	if err == nil {
		t.Fatal("Append() error = nil, want byte-limit error")
	}
}

func TestResponsesEventBufferBoundsEventCount(t *testing.T) {
	buffer := responsesEventBuffer{
		events: make([]llm.StreamEvent, maxBufferedResponsesEvents),
	}
	if err := buffer.Append(llm.StreamEvent{Type: llm.StreamEventResponseStop}); err == nil {
		t.Fatal("Append() error = nil, want event-limit error")
	}
}
