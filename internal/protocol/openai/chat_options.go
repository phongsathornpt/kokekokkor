package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeChatToolChoice(raw json.RawMessage) (*llm.ToolChoice, error) {
	var mode string
	if err := json.Unmarshal(raw, &mode); err == nil {
		switch mode {
		case "auto":
			return &llm.ToolChoice{Mode: llm.ToolChoiceAuto}, nil
		case "required":
			return &llm.ToolChoice{Mode: llm.ToolChoiceRequired}, nil
		case "none":
			return &llm.ToolChoice{Mode: llm.ToolChoiceNone}, nil
		}
	}
	var named struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &named); err != nil || named.Type != "function" || named.Function.Name == "" {
		return nil, fmt.Errorf("unsupported chat tool_choice")
	}
	return &llm.ToolChoice{Mode: llm.ToolChoiceNamed, Name: named.Function.Name}, nil
}

func encodeChatToolChoice(choice *llm.ToolChoice) (json.RawMessage, error) {
	if choice == nil {
		return nil, nil
	}
	switch choice.Mode {
	case llm.ToolChoiceAuto, llm.ToolChoiceRequired, llm.ToolChoiceNone:
		return json.Marshal(string(choice.Mode))
	case llm.ToolChoiceNamed:
		return json.Marshal(map[string]any{"type": "function", "function": map[string]string{"name": choice.Name}})
	default:
		return nil, fmt.Errorf("unsupported canonical tool choice %q", choice.Mode)
	}
}

func decodeChatStop(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, fmt.Errorf("decode chat stop: %w", err)
	}
	return many, nil
}

func encodeChatStop(stop []string) (json.RawMessage, error) {
	if len(stop) == 0 {
		return nil, nil
	}
	if len(stop) == 1 {
		return json.Marshal(stop[0])
	}
	return json.Marshal(stop)
}
