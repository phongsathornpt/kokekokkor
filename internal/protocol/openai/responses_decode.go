package openai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func DecodeResponsesRequest(data []byte) (llm.Request, error) {
	var wire responsesRequest
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Request{}, fmt.Errorf("decode OpenAI Responses request: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return llm.Request{}, fmt.Errorf("decode OpenAI Responses metadata: %w", err)
	}
	rawTools := cloneRaw(raw["tools"])
	for _, key := range []string{
		"model", "instructions", "input", "tools", "tool_choice",
		"parallel_tool_calls", "max_output_tokens", "temperature", "top_p", "reasoning", "text",
		"previous_response_id", "conversation", "store",
	} {
		delete(raw, key)
	}

	request := llm.Request{
		Model:           wire.Model,
		MaxOutputTokens: wire.MaxOutputTokens,
		Temperature:     wire.Temperature,
		TopP:            wire.TopP,
		Metadata:        raw,
	}
	state, err := decodeResponsesState(wire)
	if err != nil {
		return llm.Request{}, err
	}
	request.ResponseState = state
	if wire.Reasoning != nil {
		request.Reasoning = decodeResponsesReasoning(*wire.Reasoning)
	}
	if len(wire.Text) != 0 && string(wire.Text) != "null" {
		format, extras, err := decodeResponsesTextConfig(wire.Text)
		if err != nil {
			return llm.Request{}, err
		}
		request.ResponseFormat = format
		if len(extras) != 0 {
			if request.Metadata == nil {
				request.Metadata = make(map[string]json.RawMessage)
			}
			request.Metadata["openai.responses.text"] = extras
		}
	}

	if len(wire.Instructions) != 0 && string(wire.Instructions) != "null" {
		var instructions string
		if err := json.Unmarshal(wire.Instructions, &instructions); err != nil {
			return llm.Request{}, fmt.Errorf("Responses instructions must be a string: %w", err)
		}
		if instructions != "" {
			request.Messages = append(request.Messages, llm.Message{
				Role:    llm.RoleDeveloper,
				Content: []llm.ContentBlock{llm.TextBlock{Text: instructions}},
			})
			if request.ResponseState == nil {
				request.ResponseState = &llm.ResponseState{}
			}
			request.ResponseState.InstructionMessages = 1
		}
	}

	messages, err := decodeResponsesInput(wire.Input)
	if err != nil {
		return llm.Request{}, err
	}
	request.Messages = append(request.Messages, messages...)

	var rawToolObjects []map[string]json.RawMessage
	if len(rawTools) != 0 && string(rawTools) != "null" {
		if err := json.Unmarshal(rawTools, &rawToolObjects); err != nil {
			return llm.Request{}, fmt.Errorf("decode Responses tool metadata: %w", err)
		}
	}
	for i, tool := range wire.Tools {
		switch tool.Type {
		case "function":
			if tool.Name == "" || len(tool.Parameters) == 0 {
				return llm.Request{}, fmt.Errorf("Responses function tool %d is missing name or parameters", i)
			}
			canonical := llm.Tool{
				Kind:        llm.ToolKindFunction,
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: cloneRaw(tool.Parameters),
			}
			if tool.Strict {
				canonical.Metadata = map[string]json.RawMessage{"openai.strict": json.RawMessage("true")}
			}
			request.Tools = append(request.Tools, canonical)
		case "web_search":
			if i >= len(rawToolObjects) {
				return llm.Request{}, fmt.Errorf("Responses web_search tool %d metadata is missing", i)
			}
			if extras := responseToolExtras(rawToolObjects[i], "type"); len(extras) != 0 {
				return llm.Request{}, fmt.Errorf("Responses web_search tool %d has unsupported options: %s", i, strings.Join(extras, ", "))
			}
			request.Tools = append(request.Tools, llm.Tool{Kind: llm.ToolKindWebSearch})
		default:
			return llm.Request{}, fmt.Errorf("Responses tool %d has unsupported type %q", i, tool.Type)
		}
	}

	if len(wire.ToolChoice) != 0 && string(wire.ToolChoice) != "null" {
		choice, err := decodeResponsesToolChoice(wire.ToolChoice)
		if err != nil {
			return llm.Request{}, err
		}
		request.ToolChoice = choice
	}
	if wire.ParallelToolCalls != nil && !*wire.ParallelToolCalls {
		if request.ToolChoice == nil {
			request.ToolChoice = &llm.ToolChoice{Mode: llm.ToolChoiceAuto}
		}
		request.ToolChoice.DisableParallel = true
	}

	if err := request.Validate(); err != nil {
		return llm.Request{}, fmt.Errorf("validate OpenAI Responses request: %w", err)
	}
	return request, nil
}

func decodeResponsesReasoning(wire responseReasoningConfig) *llm.ReasoningConfig {
	summary := wire.Summary
	if summary == "" {
		summary = wire.GenerateSummary
	}
	reasoning := &llm.ReasoningConfig{
		Enabled: wire.Effort != "none",
		Effort:  wire.Effort,
		Summary: summary,
	}
	if wire.Context != "" || wire.Mode != "" {
		reasoning.Metadata = make(map[string]json.RawMessage, 2)
		if wire.Context != "" {
			reasoning.Metadata["openai.context"], _ = json.Marshal(wire.Context)
		}
		if wire.Mode != "" {
			reasoning.Metadata["openai.mode"], _ = json.Marshal(wire.Mode)
		}
	}
	return reasoning
}

func ResponsesRequestStreams(data []byte) (bool, error) {
	var request struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return false, fmt.Errorf("decode Responses stream flag: %w", err)
	}
	return request.Stream, nil
}

func decodeResponsesInput(raw json.RawMessage) ([]llm.Message, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, err
		}
		return []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: text}},
		}}, nil
	}
	if trimmed[0] != '[' {
		return nil, fmt.Errorf("Responses input must be a string or item array")
	}

	var items []responseInputItem
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, fmt.Errorf("decode Responses input items: %w", err)
	}
	messages := make([]llm.Message, 0, len(items))
	for i, item := range items {
		decoded, err := decodeResponseInputItem(item)
		if err != nil {
			return nil, fmt.Errorf("Responses input item %d: %w", i, err)
		}
		messages = append(messages, decoded...)
	}
	return messages, nil
}

func decodeResponseInputItem(item responseInputItem) ([]llm.Message, error) {
	switch item.Type {
	case "", "message":
		role, err := decodeResponsesRole(item.Role)
		if err != nil {
			return nil, err
		}
		content, err := decodeResponsesContent(item.Content)
		if err != nil {
			return nil, err
		}
		return []llm.Message{{Role: role, Content: content}}, nil

	case "function_call":
		arguments := json.RawMessage(item.Arguments)
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}
		if !json.Valid(arguments) {
			return nil, fmt.Errorf("function_call arguments are not valid JSON")
		}
		return []llm.Message{{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{llm.ToolCallBlock{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: cloneRaw(arguments),
			}},
		}}, nil

	case "function_call_output":
		content, err := decodeFunctionCallOutput(item.Output)
		if err != nil {
			return nil, err
		}
		return []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{llm.ToolResultBlock{
				ToolCallID: item.CallID,
				Content:    content,
			}},
		}}, nil

	default:
		return nil, fmt.Errorf("unsupported Responses input item type %q", item.Type)
	}
}

func decodeResponsesContent(raw json.RawMessage) ([]llm.ContentBlock, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, err
		}
		return []llm.ContentBlock{llm.TextBlock{Text: text}}, nil
	}

	var parts []responseContentPart
	if err := json.Unmarshal(trimmed, &parts); err != nil {
		return nil, fmt.Errorf("decode Responses message content: %w", err)
	}
	blocks := make([]llm.ContentBlock, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text":
			blocks = append(blocks, llm.TextBlock{Text: part.Text})
		case "input_image":
			if part.FileID != "" {
				blocks = append(blocks, llm.ImageBlock{Source: llm.MediaSource{Type: llm.MediaSourceFile, FileID: part.FileID}})
				continue
			}
			if part.ImageURL == "" {
				return nil, fmt.Errorf("input_image requires image_url or file_id")
			}
			blocks = append(blocks, llm.ImageBlock{Source: decodeImageURL(part.ImageURL)})
		case "input_file":
			switch {
			case part.FileData != "":
				mediaType, data, ok := parseDataURL(part.FileData)
				if !ok {
					return nil, fmt.Errorf("input_file file_data must be a base64 data URL")
				}
				if _, err := base64.StdEncoding.DecodeString(data); err != nil {
					return nil, fmt.Errorf("input_file file_data contains invalid base64: %w", err)
				}
				blocks = append(blocks, llm.DocumentBlock{
					Source: llm.MediaSource{Type: llm.MediaSourceBase64, MediaType: mediaType, Data: data},
					Name:   part.Filename,
				})
			case part.FileID != "":
				blocks = append(blocks, llm.DocumentBlock{Source: llm.MediaSource{Type: llm.MediaSourceFile, FileID: part.FileID}, Name: part.Filename})
			case part.FileURL != "":
				blocks = append(blocks, llm.DocumentBlock{Source: llm.MediaSource{Type: llm.MediaSourceURL, URL: part.FileURL}, Name: part.Filename})
			default:
				return nil, fmt.Errorf("input_file requires file_data, file_id, or file_url")
			}
		default:
			return nil, fmt.Errorf("unsupported Responses content part %q", part.Type)
		}
	}
	return blocks, nil
}

func decodeFunctionCallOutput(raw json.RawMessage) ([]llm.ContentBlock, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return []llm.ContentBlock{llm.TextBlock{}}, nil
	}
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, err
		}
		return []llm.ContentBlock{llm.TextBlock{Text: text}}, nil
	}
	return decodeResponsesContent(trimmed)
}

func decodeResponsesRole(role string) (llm.Role, error) {
	switch role {
	case "system":
		return llm.RoleSystem, nil
	case "developer":
		return llm.RoleDeveloper, nil
	case "user":
		return llm.RoleUser, nil
	case "assistant":
		return llm.RoleAssistant, nil
	default:
		return "", fmt.Errorf("unsupported Responses message role %q", role)
	}
}

func decodeResponsesToolChoice(raw json.RawMessage) (*llm.ToolChoice, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return nil, err
		}
		switch value {
		case "auto":
			return &llm.ToolChoice{Mode: llm.ToolChoiceAuto}, nil
		case "required":
			return &llm.ToolChoice{Mode: llm.ToolChoiceRequired}, nil
		case "none":
			return &llm.ToolChoice{Mode: llm.ToolChoiceNone}, nil
		default:
			return nil, fmt.Errorf("unsupported Responses tool_choice %q", value)
		}
	}

	var choice struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(trimmed, &choice); err != nil {
		return nil, fmt.Errorf("decode Responses tool_choice: %w", err)
	}
	if choice.Type != "function" || choice.Name == "" {
		return nil, fmt.Errorf("unsupported Responses tool_choice type %q", choice.Type)
	}
	return &llm.ToolChoice{Mode: llm.ToolChoiceNamed, Name: choice.Name}, nil
}

func responseToolExtras(object map[string]json.RawMessage, known ...string) []string {
	allowed := make(map[string]struct{}, len(known))
	for _, key := range known {
		allowed[key] = struct{}{}
	}
	extras := make([]string, 0)
	for key := range object {
		if _, ok := allowed[key]; !ok {
			extras = append(extras, key)
		}
	}
	sort.Strings(extras)
	return extras
}

func decodeResponsesState(wire responsesRequest) (*llm.ResponseState, error) {
	state := &llm.ResponseState{PreviousResponseID: wire.PreviousResponseID}
	if wire.Store != nil {
		state.Store = *wire.Store
	}
	if len(wire.Conversation) != 0 && string(wire.Conversation) != "null" {
		var id string
		if err := json.Unmarshal(wire.Conversation, &id); err != nil {
			var object struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(wire.Conversation, &object); err != nil || object.ID == "" {
				return nil, fmt.Errorf("Responses conversation must be a string or object with id")
			}
			id = object.ID
		}
		state.ConversationID = id
	}
	if state.PreviousResponseID != "" && state.ConversationID != "" {
		return nil, fmt.Errorf("Responses previous_response_id cannot be used with conversation")
	}
	if state.PreviousResponseID == "" && state.ConversationID == "" && wire.Store == nil {
		return nil, nil
	}
	return state, nil
}
