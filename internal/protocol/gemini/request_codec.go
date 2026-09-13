package gemini

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func GenerateContentPath(model string) (string, error) {
	model = strings.TrimSpace(strings.TrimPrefix(model, "models/"))
	if model == "" {
		return "", fmt.Errorf("Gemini model must not be empty")
	}
	return "/v1beta/models/" + url.PathEscape(model) + ":generateContent", nil
}

func DecodeGenerateContentRequest(data []byte) (llm.Request, error) {
	var wire generateContentRequest
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Request{}, fmt.Errorf("decode Gemini generateContent request: %w", err)
	}

	metadata, err := topLevelMetadata(data, "contents", "tools", "toolConfig", "systemInstruction", "generationConfig")
	if err != nil {
		return llm.Request{}, err
	}
	if raw := nestedExtras(data, "generationConfig", "maxOutputTokens", "temperature", "topP", "stopSequences"); len(raw) != 0 {
		metadata = putMetadata(metadata, "gemini.generationConfig", raw)
	}
	if raw := nestedExtras(data, "toolConfig", "functionCallingConfig"); len(raw) != 0 {
		metadata = putMetadata(metadata, "gemini.toolConfig", raw)
	}

	request := llm.Request{Metadata: metadata}
	if wire.GenerationConfig != nil {
		request.MaxOutputTokens = wire.GenerationConfig.MaxOutputTokens
		request.Temperature = wire.GenerationConfig.Temperature
		request.TopP = wire.GenerationConfig.TopP
		request.Stop = append([]string(nil), wire.GenerationConfig.StopSequences...)
	}
	if wire.SystemInstruction != nil {
		content, messageMetadata, err := decodeGeminiParts(wire.SystemInstruction.Parts, 0, nil)
		if err != nil {
			return llm.Request{}, fmt.Errorf("decode Gemini systemInstruction: %w", err)
		}
		request.Messages = append(request.Messages, llm.Message{Role: llm.RoleSystem, Content: content, Metadata: messageMetadata})
	}

	callNames := make(map[string]string)
	for i, content := range wire.Contents {
		role, err := decodeGeminiRole(content.Role)
		if err != nil {
			return llm.Request{}, fmt.Errorf("content %d: %w", i, err)
		}
		blocks, messageMetadata, err := decodeGeminiParts(content.Parts, i+1, callNames)
		if err != nil {
			return llm.Request{}, fmt.Errorf("content %d: %w", i, err)
		}
		request.Messages = append(request.Messages, llm.Message{Role: role, Content: blocks, Metadata: messageMetadata})
	}

	for _, tool := range wire.Tools {
		for _, function := range tool.FunctionDeclarations {
			schema := function.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object"}`)
			}
			request.Tools = append(request.Tools, llm.Tool{
				Name:        function.Name,
				Description: function.Description,
				InputSchema: append(json.RawMessage(nil), schema...),
			})
		}
	}
	if wire.ToolConfig != nil && wire.ToolConfig.FunctionCallingConfig != nil {
		choice, err := decodeGeminiToolChoice(*wire.ToolConfig.FunctionCallingConfig)
		if err != nil {
			return llm.Request{}, err
		}
		request.ToolChoice = choice
	}
	if err := request.Validate(); err != nil {
		return llm.Request{}, fmt.Errorf("validate Gemini request: %w", err)
	}
	return request, nil
}

func EncodeGenerateContentRequest(request llm.Request) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("validate canonical request: %w", err)
	}

	wire := generateContentRequest{}
	callNames := make(map[string]string)
	for _, message := range request.Messages {
		parts, err := encodeGeminiParts(message.Content, callNames)
		if err != nil {
			return nil, err
		}
		switch message.Role {
		case llm.RoleSystem:
			if wire.SystemInstruction == nil {
				wire.SystemInstruction = &geminiContent{}
			}
			wire.SystemInstruction.Parts = append(wire.SystemInstruction.Parts, parts...)
		case llm.RoleUser:
			wire.Contents = append(wire.Contents, geminiContent{Role: "user", Parts: parts})
		case llm.RoleAssistant:
			wire.Contents = append(wire.Contents, geminiContent{Role: "model", Parts: parts})
		default:
			return nil, fmt.Errorf("canonical role %q cannot be encoded as Gemini content", message.Role)
		}
	}
	if len(request.Tools) != 0 {
		functions := make([]geminiFunctionDeclaration, 0, len(request.Tools))
		for _, tool := range request.Tools {
			functions = append(functions, geminiFunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  append(json.RawMessage(nil), tool.InputSchema...),
			})
		}
		wire.Tools = []geminiTool{{FunctionDeclarations: functions}}
	}
	if request.ToolChoice != nil {
		config, err := encodeGeminiToolChoice(*request.ToolChoice)
		if err != nil {
			return nil, err
		}
		wire.ToolConfig = &geminiToolConfig{FunctionCallingConfig: &config}
	}
	if request.MaxOutputTokens != nil || request.Temperature != nil || request.TopP != nil || len(request.Stop) != 0 {
		wire.GenerationConfig = &geminiGenerationConfig{
			MaxOutputTokens: request.MaxOutputTokens,
			Temperature:     request.Temperature,
			TopP:            request.TopP,
			StopSequences:   append([]string(nil), request.Stop...),
		}
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode Gemini generateContent request: %w", err)
	}
	return data, nil
}

func decodeGeminiRole(role string) (llm.Role, error) {
	switch role {
	case "", "user":
		return llm.RoleUser, nil
	case "model":
		return llm.RoleAssistant, nil
	default:
		return "", fmt.Errorf("unsupported Gemini role %q", role)
	}
}

func decodeGeminiToolChoice(config geminiFunctionCallingConfig) (*llm.ToolChoice, error) {
	switch strings.ToUpper(config.Mode) {
	case "", "AUTO":
		return &llm.ToolChoice{Mode: llm.ToolChoiceAuto}, nil
	case "NONE":
		return &llm.ToolChoice{Mode: llm.ToolChoiceNone}, nil
	case "ANY":
		if len(config.AllowedFunctionNames) == 1 {
			return &llm.ToolChoice{Mode: llm.ToolChoiceNamed, Name: config.AllowedFunctionNames[0]}, nil
		}
		if len(config.AllowedFunctionNames) == 0 {
			return &llm.ToolChoice{Mode: llm.ToolChoiceRequired}, nil
		}
		return nil, fmt.Errorf("Gemini allowedFunctionNames with multiple names has no canonical tool-choice equivalent")
	default:
		return nil, fmt.Errorf("unsupported Gemini function calling mode %q", config.Mode)
	}
}

func encodeGeminiToolChoice(choice llm.ToolChoice) (geminiFunctionCallingConfig, error) {
	switch choice.Mode {
	case llm.ToolChoiceAuto:
		return geminiFunctionCallingConfig{Mode: "AUTO"}, nil
	case llm.ToolChoiceRequired:
		return geminiFunctionCallingConfig{Mode: "ANY"}, nil
	case llm.ToolChoiceNone:
		return geminiFunctionCallingConfig{Mode: "NONE"}, nil
	case llm.ToolChoiceNamed:
		return geminiFunctionCallingConfig{Mode: "ANY", AllowedFunctionNames: []string{choice.Name}}, nil
	default:
		return geminiFunctionCallingConfig{}, fmt.Errorf("unsupported canonical tool choice %q for Gemini", choice.Mode)
	}
}
