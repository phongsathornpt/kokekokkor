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
	Model            string             `json:"model"`
	Instructions     string             `json:"instructions"`
	Modalities       []string           `json:"modalities"`
	OutputModalities []string           `json:"output_modalities"`
	Tools            []realtimeToolWire `json:"tools"`
	ToolChoice       json.RawMessage    `json:"tool_choice"`
}

type realtimeToolWire struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type realtimeNamedToolChoiceWire struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

type realtimeItemWire struct {
	Type    string            `json:"type"`
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
	CallID  string            `json:"call_id"`
	Output  string            `json:"output"`
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
		message, toolResult, err := decodeRealtimeItem(wire.Item)
		if err != nil {
			return llm.RealtimeEvent{}, err
		}
		event.Message = message
		event.ToolResult = toolResult
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
	delete(fields, "tools")
	delete(fields, "tool_choice")

	modalities := append([]string(nil), wire.OutputModalities...)
	if len(modalities) == 0 {
		modalities = append(modalities, wire.Modalities...)
	}
	session := &llm.RealtimeSessionConfig{
		Model:            wire.Model,
		Instructions:     wire.Instructions,
		OutputModalities: modalities,
	}
	for _, tool := range wire.Tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("decode OpenAI Realtime session: unsupported tool type %q", tool.Type)
		}
		if tool.Name == "" || len(tool.Parameters) == 0 || !json.Valid(tool.Parameters) {
			return nil, fmt.Errorf("decode OpenAI Realtime session: invalid function tool")
		}
		session.Tools = append(session.Tools, llm.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: cloneRawMessage(tool.Parameters),
		})
	}
	if len(wire.ToolChoice) != 0 && string(wire.ToolChoice) != "null" {
		choice, err := decodeRealtimeToolChoice(wire.ToolChoice)
		if err != nil {
			return nil, err
		}
		session.ToolChoice = choice
	}
	if len(fields) != 0 {
		session.Metadata = fields
	}
	return session, nil
}

func decodeRealtimeToolChoice(data json.RawMessage) (*llm.ToolChoice, error) {
	var mode string
	if err := json.Unmarshal(data, &mode); err == nil {
		switch mode {
		case "auto":
			return &llm.ToolChoice{Mode: llm.ToolChoiceAuto}, nil
		case "required":
			return &llm.ToolChoice{Mode: llm.ToolChoiceRequired}, nil
		case "none":
			return &llm.ToolChoice{Mode: llm.ToolChoiceNone}, nil
		default:
			return nil, fmt.Errorf("decode OpenAI Realtime tool_choice: unsupported mode %q", mode)
		}
	}

	var wire realtimeNamedToolChoiceWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decode OpenAI Realtime tool_choice: %w", err)
	}
	if wire.Type != "function" {
		return nil, fmt.Errorf("decode OpenAI Realtime tool_choice: unsupported type %q", wire.Type)
	}
	name := wire.Name
	if name == "" {
		name = wire.Function.Name
	}
	if name == "" {
		return nil, fmt.Errorf("decode OpenAI Realtime tool_choice: missing function name")
	}
	return &llm.ToolChoice{Mode: llm.ToolChoiceNamed, Name: name}, nil
}

func decodeRealtimeItem(data json.RawMessage) (*llm.Message, *llm.ToolResultBlock, error) {
	var wire realtimeItemWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, nil, fmt.Errorf("decode OpenAI Realtime item: %w", err)
	}
	if wire.Type == "function_call_output" {
		if wire.CallID == "" {
			return nil, nil, fmt.Errorf("decode OpenAI Realtime function_call_output: missing call_id")
		}
		return nil, &llm.ToolResultBlock{
			ToolCallID: wire.CallID,
			Content:    []llm.ContentBlock{llm.TextBlock{Text: wire.Output}},
		}, nil
	}
	if wire.Type != "message" {
		return nil, nil, nil
	}
	role := llm.Role(wire.Role)
	switch role {
	case llm.RoleSystem, llm.RoleDeveloper, llm.RoleUser, llm.RoleAssistant:
	default:
		return nil, nil, nil
	}

	message := &llm.Message{Role: role}
	for _, raw := range wire.Content {
		var part realtimeTextPartWire
		if err := json.Unmarshal(raw, &part); err != nil {
			return nil, nil, fmt.Errorf("decode OpenAI Realtime item content: %w", err)
		}
		switch part.Type {
		case "input_text", "text", "output_text":
			message.Content = append(message.Content, llm.TextBlock{Text: part.Text})
		default:
			return nil, nil, nil
		}
	}
	return message, nil, nil
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
