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

type realtimeSessionWire struct {
	Model            string   `json:"model"`
	Instructions     string   `json:"instructions"`
	Modalities       []string `json:"modalities"`
	OutputModalities []string `json:"output_modalities"`
}

type realtimeItemWire struct {
	Type    string            `json:"type"`
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

type realtimeTextPartWire struct {
	Type string `json:"type"`
	Text string `json:"text"`
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
	if len(wire.Session) != 0 {
		session, err := decodeRealtimeSession(wire.Session)
		if err != nil {
			return llm.RealtimeEvent{}, err
		}
		event.SessionConfig = session
	}
	if len(wire.Item) != 0 {
		message, err := decodeRealtimeMessage(wire.Item)
		if err != nil {
			return llm.RealtimeEvent{}, err
		}
		event.Message = message
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

func decodeRealtimeSession(data json.RawMessage) (*llm.RealtimeSessionConfig, error) {
	var wire realtimeSessionWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decode OpenAI Realtime session: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("decode OpenAI Realtime session fields: %w", err)
	}
	delete(fields, "model")
	delete(fields, "instructions")
	delete(fields, "modalities")
	delete(fields, "output_modalities")

	modalities := append([]string(nil), wire.OutputModalities...)
	if len(modalities) == 0 {
		modalities = append(modalities, wire.Modalities...)
	}
	session := &llm.RealtimeSessionConfig{
		Model:            wire.Model,
		Instructions:     wire.Instructions,
		OutputModalities: modalities,
	}
	if len(fields) != 0 {
		session.Metadata = fields
	}
	return session, nil
}

func decodeRealtimeMessage(data json.RawMessage) (*llm.Message, error) {
	var wire realtimeItemWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decode OpenAI Realtime item: %w", err)
	}
	if wire.Type != "message" {
		return nil, nil
	}
	role := llm.Role(wire.Role)
	switch role {
	case llm.RoleSystem, llm.RoleDeveloper, llm.RoleUser, llm.RoleAssistant:
	default:
		return nil, nil
	}

	message := &llm.Message{Role: role}
	for _, raw := range wire.Content {
		var part realtimeTextPartWire
		if err := json.Unmarshal(raw, &part); err != nil {
			return nil, fmt.Errorf("decode OpenAI Realtime item content: %w", err)
		}
		switch part.Type {
		case "input_text", "text", "output_text":
			message.Content = append(message.Content, llm.TextBlock{Text: part.Text})
		default:
			return nil, nil
		}
	}
	return message, nil
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
