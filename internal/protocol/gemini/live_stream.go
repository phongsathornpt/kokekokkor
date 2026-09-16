package gemini

import (
	"errors"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

// LiveStreamDecoder converts the portable text subset of Gemini Live server
// messages into canonical stream events. Control-plane messages such as
// setupComplete and goAway are intentionally handled by the session bridge.
type LiveStreamDecoder struct {
	model        string
	responseOpen bool
	contentOpen  bool
}

func NewLiveStreamDecoder(model string) *LiveStreamDecoder {
	return &LiveStreamDecoder{model: model}
}

func (d *LiveStreamDecoder) Decode(message LiveServerMessage) ([]llm.StreamEvent, error) {
	if len(message.Metadata) != 0 {
		return nil, errors.New("Gemini Live server message contains provider-specific metadata")
	}
	if len(message.ToolCall) != 0 || len(message.ToolCallCancellation) != 0 {
		return nil, errors.New("Gemini Live tool events are not supported by the text realtime bridge")
	}
	if len(message.GoAway) != 0 || len(message.SessionResumptionUpdate) != 0 || message.SetupComplete {
		if message.ServerContent == nil && message.Usage == nil {
			return nil, nil
		}
	}

	if message.ServerContent != nil && len(message.ServerContent.Metadata) != 0 {
		return nil, errors.New("Gemini Live server content contains provider-specific metadata")
	}
	if message.Usage != nil && len(message.Usage.Metadata) != 0 {
		return nil, errors.New("Gemini Live usage contains provider-specific metadata")
	}

	var events []llm.StreamEvent
	content := message.ServerContent
	if content != nil && (len(content.Text) != 0 || content.GenerationComplete || content.TurnComplete || content.Interrupted) {
		if !d.responseOpen {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStart, Model: d.model})
			d.responseOpen = true
		}
		if len(content.Text) != 0 && !d.contentOpen {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}})
			d.contentOpen = true
		}
		for _, text := range content.Text {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: text})
		}
	}

	if message.Usage != nil {
		if !d.responseOpen {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStart, Model: d.model})
			d.responseOpen = true
		}
		usage := *message.Usage
		events = append(events, llm.StreamEvent{Type: llm.StreamEventUsage, Usage: &usage})
	}

	if content != nil && (content.TurnComplete || content.Interrupted) {
		if d.contentOpen {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventContentStop, Index: 0})
			d.contentOpen = false
		}
		stopReason := llm.StopReasonEndTurn
		if content.Interrupted {
			stopReason = llm.StopReasonUnknown
		}
		events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: stopReason})
		d.responseOpen = false
	}

	if content == nil && message.Usage == nil && len(events) == 0 {
		return nil, fmt.Errorf("Gemini Live server message has no portable realtime content")
	}
	return events, nil
}
