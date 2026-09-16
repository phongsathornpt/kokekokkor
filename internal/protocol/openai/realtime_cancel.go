package openai

import (
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

// EncodeCancelledResponse closes an active Realtime response after an upstream
// interruption. Any open content must be stopped first so the output snapshot
// contains only bytes that were actually delivered to the client.
func (e *RealtimeServerEncoder) EncodeCancelledResponse(event llm.StreamEvent) ([][]byte, error) {
	if event.Type != llm.StreamEventResponseStop || event.StopReason != llm.StopReasonCancelled {
		return nil, fmt.Errorf("cancelled response encoder requires a cancelled response_stop event")
	}
	if !e.started || e.stopped || e.contentOpen {
		return nil, fmt.Errorf("cancelled response_stop outside valid OpenAI Realtime response state")
	}
	e.stopped = true
	return e.marshalEvents(map[string]any{
		"type":     "response.done",
		"event_id": e.nextID("event"),
		"response": e.responseSnapshot("cancelled", nil),
	})
}
