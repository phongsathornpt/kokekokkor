package gemini

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

var (
	errLiveSetupRequired = errors.New("Gemini Live setup must be sent before content")
	errLiveSetupAlready  = errors.New("Gemini Live setup is immutable after the first message")
)

// LiveClientEncoder converts the canonical text/function realtime subset into
// Gemini Live client messages while enforcing Gemini's session ordering rules.
type LiveClientEncoder struct {
	model       string
	setupSent   bool
	contentSent bool
}

func NewLiveClientEncoder(model string) *LiveClientEncoder {
	return &LiveClientEncoder{model: model}
}

func (e *LiveClientEncoder) Encode(event llm.RealtimeEvent) ([]byte, error) {
	switch event.Type {
	case llm.RealtimeEventSessionStart, llm.RealtimeEventSessionUpdate:
		return e.encodeSetup(event)
	case llm.RealtimeEventItemCreate:
		if event.ToolResult != nil {
			return e.encodeToolResult(event.ToolResult)
		}
		return e.encodeMessage(event)
	case llm.RealtimeEventResponseCreate:
		return e.encodeTurnComplete()
	default:
		return nil, fmt.Errorf("Gemini Live client encoder: unsupported realtime event %q", event.Type)
	}
}

func (e *LiveClientEncoder) encodeSetup(event llm.RealtimeEvent) ([]byte, error) {
	if e.setupSent || e.contentSent {
		return nil, errLiveSetupAlready
	}
	if event.SessionConfig == nil {
		return nil, errors.New("Gemini Live setup requires canonical session configuration")
	}
	config := event.SessionConfig
	if len(config.Metadata) != 0 {
		return nil, errors.New("Gemini Live setup cannot encode provider-specific session metadata")
	}
	if len(config.OutputModalities) != 1 || config.OutputModalities[0] != "text" {
		return nil, errors.New("Gemini Live text encoder requires exactly one text output modality")
	}

	model := strings.TrimSpace(e.model)
	if model == "" {
		model = strings.TrimSpace(config.Model)
	}
	if model == "" {
		return nil, errors.New("Gemini Live setup requires a model")
	}
	if !strings.HasPrefix(model, "models/") {
		model = "models/" + model
	}

	setup := map[string]any{
		"model":              model,
		"responseModalities": []string{"TEXT"},
	}
	if config.Instructions != "" {
		setup["systemInstruction"] = map[string]any{
			"parts": []map[string]string{{"text": config.Instructions}},
		}
	}
	if len(config.Tools) != 0 {
		declarations := make([]map[string]any, 0, len(config.Tools))
		for _, tool := range config.Tools {
			if tool.Name == "" || len(tool.InputSchema) == 0 || !json.Valid(tool.InputSchema) {
				return nil, errors.New("Gemini Live function tool requires a name and valid JSON schema")
			}
			declaration := map[string]any{
				"name":       tool.Name,
				"parameters": json.RawMessage(tool.InputSchema),
			}
			if tool.Description != "" {
				declaration["description"] = tool.Description
			}
			declarations = append(declarations, declaration)
		}
		setup["tools"] = []map[string]any{{"functionDeclarations": declarations}}
	}
	if config.ToolChoice != nil {
		functionConfig := map[string]any{}
		switch config.ToolChoice.Mode {
		case llm.ToolChoiceAuto:
			functionConfig["mode"] = "AUTO"
		case llm.ToolChoiceRequired:
			functionConfig["mode"] = "ANY"
		case llm.ToolChoiceNone:
			functionConfig["mode"] = "NONE"
		case llm.ToolChoiceNamed:
			functionConfig["mode"] = "ANY"
			functionConfig["allowedFunctionNames"] = []string{config.ToolChoice.Name}
		default:
			return nil, fmt.Errorf("Gemini Live setup: unsupported tool choice %q", config.ToolChoice.Mode)
		}
		setup["toolConfig"] = map[string]any{"functionCallingConfig": functionConfig}
	}

	payload, err := json.Marshal(map[string]any{"setup": setup})
	if err != nil {
		return nil, fmt.Errorf("encode Gemini Live setup: %w", err)
	}
	e.setupSent = true
	return payload, nil
}

func (e *LiveClientEncoder) encodeMessage(event llm.RealtimeEvent) ([]byte, error) {
	if !e.setupSent {
		return nil, errLiveSetupRequired
	}
	if event.Message == nil {
		return nil, errors.New("Gemini Live client content requires a canonical message")
	}
	if len(event.Message.Metadata) != 0 {
		return nil, errors.New("Gemini Live client content cannot encode provider-specific message metadata")
	}

	role := ""
	switch event.Message.Role {
	case llm.RoleUser:
		role = "user"
	case llm.RoleAssistant:
		role = "model"
	default:
		return nil, fmt.Errorf("Gemini Live client content: unsupported role %q", event.Message.Role)
	}

	parts := make([]map[string]string, 0, len(event.Message.Content))
	for _, block := range event.Message.Content {
		text, ok := block.(llm.TextBlock)
		if !ok {
			return nil, fmt.Errorf("Gemini Live client content: unsupported content block %T", block)
		}
		parts = append(parts, map[string]string{"text": text.Text})
	}
	if len(parts) == 0 {
		return nil, errors.New("Gemini Live client content requires at least one text part")
	}

	payload, err := json.Marshal(map[string]any{
		"clientContent": map[string]any{
			"turns": []map[string]any{{
				"role":  role,
				"parts": parts,
			}},
			"turnComplete": false,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Gemini Live client content: %w", err)
	}
	e.contentSent = true
	return payload, nil
}

func (e *LiveClientEncoder) encodeToolResult(result *llm.ToolResultBlock) ([]byte, error) {
	if !e.setupSent {
		return nil, errLiveSetupRequired
	}
	if result == nil || result.ToolCallID == "" || result.Name == "" {
		return nil, errors.New("Gemini Live tool response requires call id and function name")
	}
	if len(result.Content) != 1 {
		return nil, errors.New("Gemini Live tool response requires one text result")
	}
	text, ok := result.Content[0].(llm.TextBlock)
	if !ok {
		return nil, fmt.Errorf("Gemini Live tool response: unsupported content block %T", result.Content[0])
	}

	response := map[string]any{"output": text.Text}
	var decoded any
	if json.Unmarshal([]byte(text.Text), &decoded) == nil {
		if object, ok := decoded.(map[string]any); ok {
			response = object
		} else {
			response = map[string]any{"output": decoded}
		}
	}
	if result.IsError {
		response = map[string]any{"error": text.Text}
	}

	payload, err := json.Marshal(map[string]any{
		"toolResponse": map[string]any{
			"functionResponses": []map[string]any{{
				"id":       result.ToolCallID,
				"name":     result.Name,
				"response": response,
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Gemini Live tool response: %w", err)
	}
	e.contentSent = true
	return payload, nil
}

func (e *LiveClientEncoder) encodeTurnComplete() ([]byte, error) {
	if !e.setupSent {
		return nil, errLiveSetupRequired
	}
	payload, err := json.Marshal(map[string]any{
		"clientContent": map[string]any{
			"turnComplete": true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Gemini Live turn completion: %w", err)
	}
	e.contentSent = true
	return payload, nil
}
