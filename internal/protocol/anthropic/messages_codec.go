package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type messagesRequest struct {
	Model        string             `json:"model"`
	MaxTokens    *int               `json:"max_tokens,omitempty"`
	System       json.RawMessage    `json:"system,omitempty"`
	Messages     []anthropicMessage `json:"messages"`
	Tools        []anthropicTool    `json:"tools,omitempty"`
	ToolChoice   json.RawMessage    `json:"tool_choice,omitempty"`
	Thinking     json.RawMessage    `json:"thinking,omitempty"`
	OutputConfig json.RawMessage    `json:"output_config,omitempty"`
	Stop         []string           `json:"stop_sequences,omitempty"`
	Temperature  *float64           `json:"temperature,omitempty"`
	TopP         *float64           `json:"top_p,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
	Data      string          `json:"data,omitempty"`
	Source    json.RawMessage `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func DecodeMessagesRequest(data []byte) (llm.Request, error) {
	var wire messagesRequest
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Request{}, fmt.Errorf("decode Anthropic Messages request: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return llm.Request{}, fmt.Errorf("decode Anthropic Messages metadata: %w", err)
	}
	for _, key := range []string{"model", "max_tokens", "system", "messages", "tools", "tool_choice", "thinking", "output_config", "stop_sequences", "temperature", "top_p"} {
		delete(raw, key)
	}

	request := llm.Request{
		Model:           wire.Model,
		MaxOutputTokens: wire.MaxTokens,
		Temperature:     wire.Temperature,
		TopP:            wire.TopP,
		Stop:            append([]string(nil), wire.Stop...),
		Metadata:        raw,
	}

	if len(wire.System) != 0 && string(wire.System) != "null" {
		blocks, err := decodeAnthropicContent(wire.System)
		if err != nil {
			return llm.Request{}, fmt.Errorf("decode Anthropic system: %w", err)
		}
		request.Messages = append(request.Messages, llm.Message{Role: llm.RoleSystem, Content: blocks})
	}

	for _, message := range wire.Messages {
		role, err := decodeAnthropicRole(message.Role)
		if err != nil {
			return llm.Request{}, err
		}
		blocks, err := decodeAnthropicContent(message.Content)
		if err != nil {
			return llm.Request{}, err
		}
		request.Messages = append(request.Messages, llm.Message{Role: role, Content: blocks})
	}

	for _, tool := range wire.Tools {
		request.Tools = append(request.Tools, llm.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: cloneAnthropicRaw(tool.InputSchema),
		})
	}

	if len(wire.ToolChoice) != 0 && string(wire.ToolChoice) != "null" {
		choice, err := decodeAnthropicToolChoice(wire.ToolChoice)
		if err != nil {
			return llm.Request{}, err
		}
		request.ToolChoice = choice
	}
	if len(wire.Thinking) != 0 && string(wire.Thinking) != "null" {
		reasoning, err := decodeAnthropicThinking(wire.Thinking)
		if err != nil {
			return llm.Request{}, err
		}
		request.Reasoning = reasoning
	}
	if len(wire.OutputConfig) != 0 && string(wire.OutputConfig) != "null" {
		effort, format, extras, err := decodeAnthropicOutputConfig(wire.OutputConfig)
		if err != nil {
			return llm.Request{}, err
		}
		if effort != "" {
			if request.Reasoning == nil {
				request.Reasoning = &llm.ReasoningConfig{Enabled: true}
			}
			request.Reasoning.Effort = effort
		}
		request.ResponseFormat = format
		if len(extras) != 0 {
			if request.Metadata == nil {
				request.Metadata = make(map[string]json.RawMessage)
			}
			request.Metadata["anthropic.output_config"] = extras
		}
	}

	if err := request.Validate(); err != nil {
		return llm.Request{}, fmt.Errorf("validate Anthropic Messages request: %w", err)
	}
	return request, nil
}

func EncodeMessagesRequest(request llm.Request) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("validate canonical request: %w", err)
	}
	if request.MaxOutputTokens == nil {
		return nil, fmt.Errorf("Anthropic Messages requires max output tokens")
	}

	wire := messagesRequest{
		Model:       request.Model,
		MaxTokens:   request.MaxOutputTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stop:        append([]string(nil), request.Stop...),
	}

	var systemBlocks []llm.ContentBlock
	for _, message := range request.Messages {
		switch message.Role {
		case llm.RoleSystem, llm.RoleDeveloper:
			systemBlocks = append(systemBlocks, message.Content...)
		case llm.RoleUser, llm.RoleAssistant:
			content, err := encodeAnthropicContent(message.Content)
			if err != nil {
				return nil, err
			}
			wire.Messages = append(wire.Messages, anthropicMessage{Role: string(message.Role), Content: content})
		default:
			return nil, fmt.Errorf("unsupported canonical role %q for Anthropic", message.Role)
		}
	}
	if len(systemBlocks) != 0 {
		system, err := encodeAnthropicContent(systemBlocks)
		if err != nil {
			return nil, err
		}
		wire.System = system
	}

	for _, tool := range request.Tools {
		wire.Tools = append(wire.Tools, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: cloneAnthropicRaw(tool.InputSchema),
		})
	}

	var err error
	wire.ToolChoice, err = encodeAnthropicToolChoice(request.ToolChoice)
	if err != nil {
		return nil, err
	}
	wire.Thinking, err = encodeAnthropicThinking(request.Reasoning)
	if err != nil {
		return nil, err
	}
	wire.OutputConfig, err = encodeAnthropicOutputConfig(request.Reasoning, request.ResponseFormat)
	if err != nil {
		return nil, err
	}

	known, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode Anthropic Messages request: %w", err)
	}
	return mergeAnthropicMetadata(known, request.Metadata)
}

func decodeAnthropicRole(role string) (llm.Role, error) {
	switch role {
	case "user":
		return llm.RoleUser, nil
	case "assistant":
		return llm.RoleAssistant, nil
	default:
		return "", fmt.Errorf("unsupported Anthropic role %q", role)
	}
}

func decodeAnthropicContent(raw json.RawMessage) ([]llm.ContentBlock, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		return []llm.ContentBlock{llm.TextBlock{Text: text}}, nil
	}

	var blocks []anthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, fmt.Errorf("decode Anthropic content: %w", err)
	}
	result := make([]llm.ContentBlock, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			result = append(result, llm.TextBlock{Text: block.Text})
		case "image":
			source, err := decodeAnthropicMediaSource(block.Source)
			if err != nil {
				return nil, err
			}
			result = append(result, llm.ImageBlock{Source: source})
		case "document":
			source, err := decodeAnthropicMediaSource(block.Source)
			if err != nil {
				return nil, err
			}
			result = append(result, llm.DocumentBlock{Source: source})
		case "tool_use":
			input := cloneAnthropicRaw(block.Input)
			if len(input) == 0 {
				input = []byte(`{}`)
			}
			result = append(result, llm.ToolCallBlock{ID: block.ID, Name: block.Name, Arguments: input})
		case "tool_result":
			content, err := decodeAnthropicContent(block.Content)
			if err != nil {
				return nil, err
			}
			result = append(result, llm.ToolResultBlock{ToolCallID: block.ToolUseID, Content: content, IsError: block.IsError})
		case "thinking":
			result = append(result, llm.ReasoningBlock{Text: block.Thinking, Signature: block.Signature})
		case "redacted_thinking":
			result = append(result, llm.ReasoningBlock{RedactedData: block.Data})
		default:
			return nil, fmt.Errorf("unsupported Anthropic content type %q", block.Type)
		}
	}
	return result, nil
}

func encodeAnthropicContent(blocks []llm.ContentBlock) (json.RawMessage, error) {
	encoded := make([]any, 0, len(blocks))
	for _, block := range blocks {
		switch value := block.(type) {
		case llm.TextBlock:
			encoded = append(encoded, map[string]any{"type": "text", "text": value.Text})
		case llm.ImageBlock:
			source, err := encodeAnthropicMediaSource(value.Source)
			if err != nil {
				return nil, err
			}
			encoded = append(encoded, map[string]any{"type": "image", "source": source})
		case llm.DocumentBlock:
			source, err := encodeAnthropicMediaSource(value.Source)
			if err != nil {
				return nil, err
			}
			encoded = append(encoded, map[string]any{"type": "document", "source": source})
		case llm.ToolCallBlock:
			var input any
			if err := json.Unmarshal(value.Arguments, &input); err != nil {
				return nil, fmt.Errorf("decode canonical tool arguments: %w", err)
			}
			encoded = append(encoded, map[string]any{"type": "tool_use", "id": value.ID, "name": value.Name, "input": input})
		case llm.ToolResultBlock:
			content, err := encodeAnthropicContent(value.Content)
			if err != nil {
				return nil, err
			}
			var decoded any
			if err := json.Unmarshal(content, &decoded); err != nil {
				return nil, err
			}
			encoded = append(encoded, map[string]any{"type": "tool_result", "tool_use_id": value.ToolCallID, "content": decoded, "is_error": value.IsError})
		case llm.ReasoningBlock:
			if value.RedactedData != "" {
				encoded = append(encoded, map[string]any{"type": "redacted_thinking", "data": value.RedactedData})
				continue
			}
			item := map[string]any{"type": "thinking", "thinking": value.Text}
			if value.Signature != "" {
				item["signature"] = value.Signature
			}
			encoded = append(encoded, item)
		default:
			return nil, fmt.Errorf("unsupported canonical content block %T", block)
		}
	}
	return json.Marshal(encoded)
}

func decodeAnthropicMediaSource(raw json.RawMessage) (llm.MediaSource, error) {
	var source struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
		URL       string `json:"url"`
		FileID    string `json:"file_id"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return llm.MediaSource{}, fmt.Errorf("decode Anthropic media source: %w", err)
	}
	switch source.Type {
	case "base64":
		return llm.MediaSource{Type: llm.MediaSourceBase64, MediaType: source.MediaType, Data: source.Data}, nil
	case "url":
		return llm.MediaSource{Type: llm.MediaSourceURL, URL: source.URL}, nil
	case "file":
		return llm.MediaSource{Type: llm.MediaSourceFile, FileID: source.FileID}, nil
	default:
		return llm.MediaSource{}, fmt.Errorf("unsupported Anthropic media source %q", source.Type)
	}
}

func encodeAnthropicMediaSource(source llm.MediaSource) (map[string]any, error) {
	switch source.Type {
	case llm.MediaSourceBase64:
		return map[string]any{"type": "base64", "media_type": source.MediaType, "data": source.Data}, nil
	case llm.MediaSourceURL:
		return map[string]any{"type": "url", "url": source.URL}, nil
	case llm.MediaSourceFile:
		return map[string]any{"type": "file", "file_id": source.FileID}, nil
	default:
		return nil, fmt.Errorf("unsupported canonical media source %q", source.Type)
	}
}

func decodeAnthropicToolChoice(raw json.RawMessage) (*llm.ToolChoice, error) {
	var choice struct {
		Type                   string `json:"type"`
		Name                   string `json:"name"`
		DisableParallelToolUse bool   `json:"disable_parallel_tool_use"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("decode Anthropic tool_choice: %w", err)
	}
	result := &llm.ToolChoice{DisableParallel: choice.DisableParallelToolUse}
	switch choice.Type {
	case "auto":
		result.Mode = llm.ToolChoiceAuto
	case "any":
		result.Mode = llm.ToolChoiceRequired
	case "none":
		result.Mode = llm.ToolChoiceNone
	case "tool":
		result.Mode = llm.ToolChoiceNamed
		result.Name = choice.Name
	default:
		return nil, fmt.Errorf("unsupported Anthropic tool_choice %q", choice.Type)
	}
	return result, nil
}

func encodeAnthropicToolChoice(choice *llm.ToolChoice) (json.RawMessage, error) {
	if choice == nil {
		return nil, nil
	}
	value := map[string]any{"disable_parallel_tool_use": choice.DisableParallel}
	switch choice.Mode {
	case llm.ToolChoiceAuto:
		value["type"] = "auto"
	case llm.ToolChoiceRequired:
		value["type"] = "any"
	case llm.ToolChoiceNone:
		value["type"] = "none"
	case llm.ToolChoiceNamed:
		value["type"] = "tool"
		value["name"] = choice.Name
	default:
		return nil, fmt.Errorf("unsupported canonical tool choice %q", choice.Mode)
	}
	return json.Marshal(value)
}

func decodeAnthropicThinking(raw json.RawMessage) (*llm.ReasoningConfig, error) {
	var thinking struct {
		Type         string `json:"type"`
		BudgetTokens int    `json:"budget_tokens"`
		Display      string `json:"display"`
	}
	if err := json.Unmarshal(raw, &thinking); err != nil {
		return nil, fmt.Errorf("decode Anthropic thinking: %w", err)
	}
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, err
	}
	delete(metadata, "type")
	delete(metadata, "budget_tokens")
	delete(metadata, "display")
	reasoning := &llm.ReasoningConfig{
		Enabled:      thinking.Type != "disabled",
		Mode:         thinking.Type,
		BudgetTokens: thinking.BudgetTokens,
		Metadata:     metadata,
	}
	switch thinking.Display {
	case "summarized":
		reasoning.Summary = "auto"
	case "omitted":
		reasoning.Summary = "none"
	}
	return reasoning, nil
}

func encodeAnthropicThinking(reasoning *llm.ReasoningConfig) (json.RawMessage, error) {
	if reasoning == nil {
		return nil, nil
	}
	value := make(map[string]json.RawMessage, len(reasoning.Metadata)+3)
	for key, raw := range reasoning.Metadata {
		value[key] = cloneAnthropicRaw(raw)
	}

	mode := reasoning.Mode
	if mode == "" {
		if reasoning.Enabled {
			mode = "enabled"
		} else {
			mode = "disabled"
		}
	}
	if mode != "enabled" && mode != "adaptive" && mode != "disabled" {
		return nil, fmt.Errorf("unsupported Anthropic thinking mode %q", mode)
	}
	encodedMode, _ := json.Marshal(mode)
	value["type"] = encodedMode
	if mode == "enabled" && reasoning.BudgetTokens > 0 {
		budget, _ := json.Marshal(reasoning.BudgetTokens)
		value["budget_tokens"] = budget
	}
	if mode != "disabled" {
		var display string
		switch reasoning.Summary {
		case "":
		case "none":
			display = "omitted"
		case "auto", "concise", "detailed":
			display = "summarized"
		default:
			return nil, fmt.Errorf("unsupported Anthropic reasoning summary %q", reasoning.Summary)
		}
		if display != "" {
			encodedDisplay, _ := json.Marshal(display)
			value["display"] = encodedDisplay
		}
	}
	return json.Marshal(value)
}

func decodeAnthropicOutputConfig(raw json.RawMessage) (string, *llm.ResponseFormat, json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", nil, nil, fmt.Errorf("decode Anthropic output_config: %w", err)
	}
	var effort string
	if value, ok := object["effort"]; ok {
		if err := json.Unmarshal(value, &effort); err != nil {
			return "", nil, nil, fmt.Errorf("decode Anthropic output_config.effort: %w", err)
		}
		delete(object, "effort")
	}
	var format *llm.ResponseFormat
	if value, ok := object["format"]; ok {
		var wire struct {
			Type   string          `json:"type"`
			Schema json.RawMessage `json:"schema"`
		}
		if err := json.Unmarshal(value, &wire); err != nil {
			return "", nil, nil, fmt.Errorf("decode Anthropic output_config.format: %w", err)
		}
		if wire.Type != "json_schema" {
			return "", nil, nil, fmt.Errorf("unsupported Anthropic output format %q", wire.Type)
		}
		format = &llm.ResponseFormat{JSONSchema: cloneAnthropicRaw(wire.Schema), Strict: true}
		delete(object, "format")
	}
	if len(object) == 0 {
		return effort, format, nil, nil
	}
	extras, err := json.Marshal(object)
	if err != nil {
		return "", nil, nil, err
	}
	return effort, format, extras, nil
}

func encodeAnthropicOutputConfig(reasoning *llm.ReasoningConfig, format *llm.ResponseFormat) (json.RawMessage, error) {
	value := make(map[string]any, 2)
	if reasoning != nil && reasoning.Effort != "" {
		switch reasoning.Effort {
		case "low", "medium", "high", "xhigh", "max":
		default:
			return nil, fmt.Errorf("unsupported Anthropic effort %q", reasoning.Effort)
		}
		value["effort"] = reasoning.Effort
	}
	if format != nil {
		value["format"] = map[string]any{"type": "json_schema", "schema": json.RawMessage(format.JSONSchema)}
	}
	if len(value) == 0 {
		return nil, nil
	}
	return json.Marshal(value)
}

func cloneAnthropicRaw(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func mergeAnthropicMetadata(known []byte, metadata map[string]json.RawMessage) ([]byte, error) {
	if len(metadata) == 0 {
		return known, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(known, &object); err != nil {
		return nil, err
	}
	for key, value := range metadata {
		if _, exists := object[key]; exists {
			continue
		}
		object[key] = cloneAnthropicRaw(value)
	}
	return json.Marshal(object)
}
