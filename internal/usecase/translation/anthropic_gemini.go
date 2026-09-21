package translation

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func AnthropicToGeminiRequest(request llm.Request) (llm.Request, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := consumeStreaming(metadata); err != nil {
		return llm.Request{}, err
	}
	if err := rejectMetadata(metadata); err != nil {
		return llm.Request{}, err
	}
	if request.Reasoning != nil {
		return llm.Request{}, unsupported("thinking", "Anthropic thinking controls are not translated to Gemini yet")
	}
	if err := rejectNestedMetadata(request); err != nil {
		return llm.Request{}, err
	}
	seenNonSystem := false
	for i, message := range request.Messages {
		if message.Role == llm.RoleSystem {
			if seenNonSystem {
				return llm.Request{}, unsupported("system message order", "Gemini systemInstruction cannot preserve interleaved system messages")
			}
			for _, block := range message.Content {
				if _, ok := block.(llm.TextBlock); !ok {
					return llm.Request{}, unsupported("system content", fmt.Sprintf("system message %d contains non-text content", i))
				}
			}
		} else {
			seenNonSystem = true
		}
		for _, block := range message.Content {
			if err := checkAnthropicBlockForGemini(block); err != nil {
				return llm.Request{}, err
			}
		}
	}
	request.Metadata = nil
	return request, nil
}

func GeminiToAnthropicRequest(request llm.Request) (llm.Request, error) {
	if err := rejectMetadata(request.Metadata); err != nil {
		return llm.Request{}, err
	}
	if request.Reasoning != nil {
		return llm.Request{}, unsupported("thinking", "Gemini thinking controls cannot yet be represented in Anthropic Messages")
	}
	if err := rejectNestedMetadata(request); err != nil {
		return llm.Request{}, err
	}
	for _, tool := range request.Tools {
		if tool.Kind == llm.ToolKindWebSearch {
			return llm.Request{}, unsupported("web search", "target protocol cannot preserve Gemini Google Search tool semantics")
		}
	}
	for _, message := range request.Messages {
		for _, block := range message.Content {
			switch value := block.(type) {
			case llm.ReasoningBlock:
				return llm.Request{}, unsupported("thinking", "Gemini thought parts cannot yet be represented in Anthropic Messages")
			case llm.ImageBlock:
				if value.Source.Type != llm.MediaSourceBase64 {
					return llm.Request{}, unsupported("file image", "Gemini fileData images cannot be forwarded to Anthropic losslessly")
				}
			case llm.DocumentBlock:
				if value.Source.Type != llm.MediaSourceBase64 {
					return llm.Request{}, unsupported("document", "Gemini fileData documents cannot be forwarded to Anthropic losslessly")
				}
			}
		}
	}
	request.Metadata = nil
	return request, nil
}

func AnthropicToGeminiResponse(response llm.Response) (llm.Response, error) {
	return portableGeminiAnthropicResponse(response, "Anthropic")
}

func GeminiToAnthropicResponse(response llm.Response) (llm.Response, error) {
	return portableGeminiAnthropicResponse(response, "Gemini")
}

func portableGeminiAnthropicResponse(response llm.Response, source string) (llm.Response, error) {
	if err := rejectMetadata(response.Metadata); err != nil {
		return llm.Response{}, err
	}
	if err := rejectMetadata(response.Usage.Metadata); err != nil {
		return llm.Response{}, err
	}
	if response.StopReason == llm.StopReasonUnknown {
		return llm.Response{}, unsupported("stop reason", source+" stop reason has no portable mapping")
	}
	for _, block := range response.Content {
		switch value := block.(type) {
		case llm.TextBlock:
			if len(value.Citations) != 0 {
				return llm.Response{}, unsupported("citations", source+" URL citations cannot be represented portably")
			}
		case llm.ToolCallBlock:
		case llm.ReasoningBlock:
			return llm.Response{}, unsupported("thinking", source+" reasoning/thought output is not translated yet")
		default:
			return llm.Response{}, unsupported("response content", fmt.Sprintf("%s block %T cannot be represented portably", source, block))
		}
	}
	response.Metadata = nil
	response.Usage.Metadata = nil
	return response, nil
}

func checkAnthropicBlockForGemini(block llm.ContentBlock) error {
	switch value := block.(type) {
	case llm.ImageBlock:
		if value.Source.Type != llm.MediaSourceBase64 {
			return unsupported("image", "Gemini translation currently accepts only inline base64 images")
		}
	case llm.DocumentBlock:
		if value.Source.Type != llm.MediaSourceBase64 {
			return unsupported("document", "Gemini translation currently accepts only inline base64 documents")
		}
	case llm.ReasoningBlock:
		return unsupported("thinking", "Anthropic thinking blocks cannot yet be translated to Gemini")
	case llm.ToolCallBlock:
		var object map[string]json.RawMessage
		if json.Unmarshal(value.Arguments, &object) != nil || object == nil {
			return unsupported("tool arguments", "Gemini function-call args must be a JSON object")
		}
	case llm.ToolResultBlock:
		if len(value.Content) != 1 {
			return unsupported("tool result", "Gemini function responses currently require one text block")
		}
		if _, ok := value.Content[0].(llm.TextBlock); !ok {
			return unsupported("tool result", "Gemini function responses currently require text content")
		}
	}
	return nil
}
