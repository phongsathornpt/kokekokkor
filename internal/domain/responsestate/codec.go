package responsestate

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type messageWire struct {
	Role    llm.Role    `json:"role"`
	Name    string      `json:"name,omitempty"`
	Content []blockWire `json:"content"`
}

type blockWire struct {
	Type       string            `json:"type"`
	Text       string            `json:"text,omitempty"`
	Citations  []llm.URLCitation `json:"citations,omitempty"`
	Source     *llm.MediaSource  `json:"source,omitempty"`
	Name       string            `json:"name,omitempty"`
	Context    string            `json:"context,omitempty"`
	ID         string            `json:"id,omitempty"`
	Arguments  json.RawMessage   `json:"arguments,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Content    []blockWire       `json:"content,omitempty"`
	IsError    bool              `json:"is_error,omitempty"`
}

func MarshalMessages(messages []llm.Message) ([]byte, error) {
	wire := make([]messageWire, 0, len(messages))
	for i, message := range messages {
		if len(message.Metadata) != 0 {
			return nil, fmt.Errorf("message %d contains nonportable metadata", i)
		}
		item := messageWire{Role: message.Role, Name: message.Name}
		for j, block := range message.Content {
			encoded, err := marshalBlock(block)
			if err != nil {
				return nil, fmt.Errorf("message %d block %d: %w", i, j, err)
			}
			item.Content = append(item.Content, encoded)
		}
		wire = append(wire, item)
	}
	return json.Marshal(wire)
}

func UnmarshalMessages(data []byte) ([]llm.Message, error) {
	var wire []messageWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decode response state messages: %w", err)
	}
	messages := make([]llm.Message, 0, len(wire))
	for i, item := range wire {
		message := llm.Message{Role: item.Role, Name: item.Name}
		for j, block := range item.Content {
			decoded, err := unmarshalBlock(block)
			if err != nil {
				return nil, fmt.Errorf("message %d block %d: %w", i, j, err)
			}
			message.Content = append(message.Content, decoded)
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func marshalBlock(block llm.ContentBlock) (blockWire, error) {
	switch value := block.(type) {
	case llm.TextBlock:
		return blockWire{Type: "text", Text: value.Text, Citations: append([]llm.URLCitation(nil), value.Citations...)}, nil
	case llm.ImageBlock:
		source := value.Source
		return blockWire{Type: "image", Source: &source}, nil
	case llm.DocumentBlock:
		source := value.Source
		return blockWire{Type: "document", Source: &source, Name: value.Name, Context: value.Context}, nil
	case llm.ToolCallBlock:
		return blockWire{Type: "tool_call", ID: value.ID, Name: value.Name, Arguments: append(json.RawMessage(nil), value.Arguments...)}, nil
	case llm.ToolResultBlock:
		wire := blockWire{Type: "tool_result", ToolCallID: value.ToolCallID, Name: value.Name, IsError: value.IsError}
		for _, nested := range value.Content {
			encoded, err := marshalBlock(nested)
			if err != nil {
				return blockWire{}, err
			}
			wire.Content = append(wire.Content, encoded)
		}
		return wire, nil
	default:
		return blockWire{}, fmt.Errorf("content block %T is not persistable response state", block)
	}
}

func unmarshalBlock(wire blockWire) (llm.ContentBlock, error) {
	switch wire.Type {
	case "text":
		return llm.TextBlock{Text: wire.Text, Citations: append([]llm.URLCitation(nil), wire.Citations...)}, nil
	case "image":
		if wire.Source == nil {
			return nil, fmt.Errorf("image state is missing source")
		}
		return llm.ImageBlock{Source: *wire.Source}, nil
	case "document":
		if wire.Source == nil {
			return nil, fmt.Errorf("document state is missing source")
		}
		return llm.DocumentBlock{Source: *wire.Source, Name: wire.Name, Context: wire.Context}, nil
	case "tool_call":
		return llm.ToolCallBlock{ID: wire.ID, Name: wire.Name, Arguments: append(json.RawMessage(nil), wire.Arguments...)}, nil
	case "tool_result":
		result := llm.ToolResultBlock{ToolCallID: wire.ToolCallID, Name: wire.Name, IsError: wire.IsError}
		for _, nested := range wire.Content {
			decoded, err := unmarshalBlock(nested)
			if err != nil {
				return nil, err
			}
			result.Content = append(result.Content, decoded)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported response state block %q", wire.Type)
	}
}
