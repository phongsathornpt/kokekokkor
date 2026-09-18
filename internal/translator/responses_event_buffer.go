package translator

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

const (
	maxBufferedResponsesEvents = 8192
	maxBufferedResponsesBytes  = 8 << 20
)

type responsesEventBuffer struct {
	events []llm.StreamEvent
	bytes  int
}

func (b *responsesEventBuffer) Append(event llm.StreamEvent) error {
	if len(b.events) >= maxBufferedResponsesEvents {
		return fmt.Errorf("buffered Responses stream exceeds %d events", maxBufferedResponsesEvents)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("measure buffered Responses stream event: %w", err)
	}
	if len(encoded) > maxBufferedResponsesBytes-b.bytes {
		return fmt.Errorf("buffered Responses stream exceeds %d bytes", maxBufferedResponsesBytes)
	}
	b.events = append(b.events, event)
	b.bytes += len(encoded)
	return nil
}

func (b *responsesEventBuffer) Events() []llm.StreamEvent {
	return b.events
}
