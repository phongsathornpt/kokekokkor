package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type realtimeEventEnvelope struct {
	Type     string          `json:"type"`
	EventID  string          `json:"event_id"`
	Session  json.RawMessage `json:"session"`
	Item     json.RawMessage `json:"item"`
	Audio    string          `json:"audio"`
	Response json.RawMessage `json:"response"`
}

func DecodeRealtimeEvent(data []byte) (llm.RealtimeEvent, error) {
	var wire realtimeEventEnvelope
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.RealtimeEvent{}, fmt.Errorf("decode OpenAI Realtime event: %w", err)
	}
	if wire.Type == "" {
		return llm.RealtimeEvent{}, fmt.Errorf("decode OpenAI Realtime event: missing type")
	}

	event := llm.RealtimeEvent{
		Type:     realtimeEventType(wire.Type),
		EventID:  wire.EventID,
		WireType: wire.Type,
		Session:  cloneRawMessage(wire.Session),
		Item:     cloneRawMessage(wire.Item),
		Audio:    wire.Audio,
		Response: cloneRawMessage(wire.Response),
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return llm.RealtimeEvent{}, fmt.Errorf("decode OpenAI Realtime event fields: %w", err)
	}
	delete(fields, "type")
	delete(fields, "event_id")
	delete(fields, "session")
	delete(fields, "item")
	delete(fields, "audio")
	delete(fields, "response")
	if len(fields) != 0 {
		event.Metadata = fields
	}
	return event, nil
}

func realtimeEventType(wireType string) llm.RealtimeEventType {
	switch wireType {
	case "session.start":
		return llm.RealtimeEventSessionStart
	case "session.update":
		return llm.RealtimeEventSessionUpdate
	case "input_audio_buffer.append", "input_audio.append":
		return llm.RealtimeEventInputAudioAppend
	case "input_audio_buffer.commit", "input_audio.commit":
		return llm.RealtimeEventInputAudioCommit
	case "input_audio_buffer.clear", "input_audio.clear":
		return llm.RealtimeEventInputAudioClear
	case "conversation.item.create", "response.item.create":
		return llm.RealtimeEventItemCreate
	case "response.create":
		return llm.RealtimeEventResponseCreate
	case "response.cancel":
		return llm.RealtimeEventResponseCancel
	default:
		return llm.RealtimeEventUnknown
	}
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}
