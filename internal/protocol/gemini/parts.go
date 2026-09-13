package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeGeminiParts(parts []geminiPart, messageIndex int, callNames map[string]string) ([]llm.ContentBlock, map[string]json.RawMessage, error) {
	blocks := make([]llm.ContentBlock, 0, len(parts))
	var metadata map[string]json.RawMessage
	for i, part := range parts {
		if part.Text != nil {
			if part.Thought {
				blocks = append(blocks, llm.ReasoningBlock{Text: *part.Text, Signature: part.ThoughtSignature})
			} else {
				blocks = append(blocks, llm.TextBlock{Text: *part.Text})
			}
			continue
		}
		if part.InlineData != nil {
			source := llm.MediaSource{Type: llm.MediaSourceBase64, MediaType: part.InlineData.MIMEType, Data: part.InlineData.Data}
			if strings.HasPrefix(part.InlineData.MIMEType, "image/") {
				blocks = append(blocks, llm.ImageBlock{Source: source})
			} else {
				blocks = append(blocks, llm.DocumentBlock{Source: source})
			}
			continue
		}
		if part.FileData != nil {
			source := llm.MediaSource{Type: llm.MediaSourceURL, URL: part.FileData.FileURI, MediaType: part.FileData.MIMEType}
			if strings.HasPrefix(part.FileData.MIMEType, "image/") {
				blocks = append(blocks, llm.ImageBlock{Source: source})
			} else {
				blocks = append(blocks, llm.DocumentBlock{Source: source})
			}
			continue
		}
		if part.FunctionCall != nil {
			id := part.FunctionCall.ID
			if id == "" {
				id = fmt.Sprintf("gemini_call_%d_%d", messageIndex, i)
			}
			args := part.FunctionCall.Args
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			if !isJSONObject(args) {
				return nil, nil, fmt.Errorf("Gemini function call %q args must be a JSON object", part.FunctionCall.Name)
			}
			blocks = append(blocks, llm.ToolCallBlock{ID: id, Name: part.FunctionCall.Name, Arguments: append(json.RawMessage(nil), args...)})
			if callNames != nil {
				callNames[part.FunctionCall.Name] = id
			}
			continue
		}
		if part.FunctionResponse != nil {
			id := part.FunctionResponse.ID
			if id == "" && callNames != nil {
				id = callNames[part.FunctionResponse.Name]
			}
			if id == "" {
				return nil, nil, fmt.Errorf("Gemini function response %q has no call id and no matching prior call", part.FunctionResponse.Name)
			}
			text := "{}"
			if len(part.FunctionResponse.Response) != 0 {
				encoded, err := compactJSON(part.FunctionResponse.Response)
				if err != nil {
					return nil, nil, fmt.Errorf("Gemini function response %q: %w", part.FunctionResponse.Name, err)
				}
				text = encoded
			}
			blocks = append(blocks, llm.ToolResultBlock{ToolCallID: id, Content: []llm.ContentBlock{llm.TextBlock{Text: text}}})
			continue
		}

		raw, _ := json.Marshal(part)
		metadata = putMetadata(metadata, fmt.Sprintf("gemini.part.%d", i), raw)
	}
	return blocks, metadata, nil
}

func encodeGeminiParts(blocks []llm.ContentBlock, callNames map[string]string) ([]geminiPart, error) {
	parts := make([]geminiPart, 0, len(blocks))
	for _, block := range blocks {
		switch value := block.(type) {
		case llm.TextBlock:
			text := value.Text
			parts = append(parts, geminiPart{Text: &text})
		case llm.ImageBlock:
			switch value.Source.Type {
			case llm.MediaSourceBase64:
				parts = append(parts, geminiPart{InlineData: &geminiBlob{MIMEType: value.Source.MediaType, Data: value.Source.Data}})
			case llm.MediaSourceURL:
				parts = append(parts, geminiPart{FileData: &geminiFileData{MIMEType: value.Source.MediaType, FileURI: value.Source.URL}})
			default:
				return nil, fmt.Errorf("image source %q cannot be encoded for Gemini", value.Source.Type)
			}
		case llm.DocumentBlock:
			switch value.Source.Type {
			case llm.MediaSourceBase64:
				parts = append(parts, geminiPart{InlineData: &geminiBlob{MIMEType: value.Source.MediaType, Data: value.Source.Data}})
			case llm.MediaSourceURL:
				parts = append(parts, geminiPart{FileData: &geminiFileData{MIMEType: value.Source.MediaType, FileURI: value.Source.URL}})
			default:
				return nil, fmt.Errorf("document source %q cannot be encoded for Gemini", value.Source.Type)
			}
		case llm.ToolCallBlock:
			if !isJSONObject(value.Arguments) {
				return nil, fmt.Errorf("tool call %q arguments must be a JSON object for Gemini", value.Name)
			}
			parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{ID: value.ID, Name: value.Name, Args: append(json.RawMessage(nil), value.Arguments...)}})
			if callNames != nil {
				callNames[value.ID] = value.Name
			}
		case llm.ToolResultBlock:
			name := ""
			if callNames != nil {
				name = callNames[value.ToolCallID]
			}
			if name == "" {
				return nil, fmt.Errorf("tool result %q has no matching tool-call name for Gemini", value.ToolCallID)
			}
			response, err := encodeToolResultResponse(value)
			if err != nil {
				return nil, err
			}
			parts = append(parts, geminiPart{FunctionResponse: &geminiFunctionResponse{ID: value.ToolCallID, Name: name, Response: response}})
		case llm.ReasoningBlock:
			text := value.Text
			parts = append(parts, geminiPart{Text: &text, Thought: true, ThoughtSignature: value.Signature})
		default:
			return nil, fmt.Errorf("content block %T cannot be encoded for Gemini", block)
		}
	}
	return parts, nil
}

func encodeToolResultResponse(result llm.ToolResultBlock) (json.RawMessage, error) {
	if len(result.Content) != 1 {
		return nil, fmt.Errorf("tool result %q must contain exactly one text block for Gemini", result.ToolCallID)
	}
	text, ok := result.Content[0].(llm.TextBlock)
	if !ok {
		return nil, fmt.Errorf("tool result %q must contain text for Gemini", result.ToolCallID)
	}
	raw := json.RawMessage(text.Text)
	if isJSONObject(raw) {
		return append(json.RawMessage(nil), raw...), nil
	}
	encoded, err := json.Marshal(map[string]string{"output": text.Text})
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func isJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 || !json.Valid(raw) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(raw, &object) == nil && object != nil
}

func compactJSON(raw json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
