package translation

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func OpenAIToGeminiRequest(request llm.Request) (llm.Request, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := consumeStreaming(metadata); err != nil {
		return llm.Request{}, err
	}
	if raw, ok := metadata["n"]; ok {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil || n != 1 {
			return llm.Request{}, unsupported("n", "Gemini generateContent translation supports one candidate")
		}
		delete(metadata, "n")
	}
	if err := rejectMetadata(metadata); err != nil {
		return llm.Request{}, err
	}
	if request.Reasoning != nil {
		return llm.Request{}, unsupported("reasoning", "reasoning controls do not yet have a lossless OpenAI-to-Gemini mapping")
	}
	if request.ResponseFormat != nil {
		return llm.Request{}, unsupported("response_format", "structured-output translation is not implemented for Gemini yet")
	}
	if request.ToolChoice != nil && request.ToolChoice.DisableParallel {
		return llm.Request{}, unsupported("parallel_tool_calls", "Gemini parallel-tool policy translation is not implemented")
	}
	if err := rejectNestedMetadata(request); err != nil {
		return llm.Request{}, err
	}

	seenNonSystem := false
	for i, message := range request.Messages {
		switch message.Role {
		case llm.RoleDeveloper:
			return llm.Request{}, unsupported("developer role", "Gemini generateContent has no distinct developer-message role")
		case llm.RoleSystem:
			if seenNonSystem {
				return llm.Request{}, unsupported("system message order", "Gemini systemInstruction cannot preserve interleaved system messages")
			}
			for _, block := range message.Content {
				if _, ok := block.(llm.TextBlock); !ok {
					return llm.Request{}, unsupported("system content", fmt.Sprintf("system message %d contains non-text content", i))
				}
			}
		default:
			seenNonSystem = true
		}
		for _, block := range message.Content {
			if err := checkOpenAIBlockForGemini(block); err != nil {
				return llm.Request{}, err
			}
		}
	}

	request.Metadata = nil
	return request, nil
}

func GeminiToOpenAIRequest(request llm.Request) (llm.Request, error) {
	if err := rejectMetadata(request.Metadata); err != nil {
		return llm.Request{}, err
	}
	if request.Reasoning != nil {
		return llm.Request{}, unsupported("thinking", "Gemini thinking controls cannot yet be represented in Chat Completions")
	}
	if request.ResponseFormat != nil {
		return llm.Request{}, unsupported("response format", "Gemini structured-output controls cannot yet be represented in Chat Completions")
	}
	if request.ToolChoice != nil && request.ToolChoice.DisableParallel {
		return llm.Request{}, unsupported("tool configuration", "Gemini parallel-tool policy cannot yet be represented in Chat Completions")
	}
	if err := rejectNestedMetadata(request); err != nil {
		return llm.Request{}, err
	}
	for _, message := range request.Messages {
		for _, block := range message.Content {
			switch value := block.(type) {
			case llm.DocumentBlock:
				return llm.Request{}, unsupported("document", "Chat Completions cannot represent Gemini document/media parts losslessly")
			case llm.ReasoningBlock:
				return llm.Request{}, unsupported("thinking", "Gemini thought parts cannot yet be represented in Chat Completions")
			case llm.ImageBlock:
				if value.Source.Type != llm.MediaSourceBase64 {
					return llm.Request{}, unsupported("file image", "Gemini fileData images cannot be forwarded to Chat Completions losslessly")
				}
			case llm.ToolResultBlock:
				if value.IsError {
					return llm.Request{}, unsupported("function response error", "Chat Completions tool messages have no equivalent Gemini error flag")
				}
				if len(value.Content) != 1 {
					return llm.Request{}, unsupported("function response", "tool results must contain one text payload for Chat Completions")
				}
				if _, ok := value.Content[0].(llm.TextBlock); !ok {
					return llm.Request{}, unsupported("function response", "tool results must contain text for Chat Completions")
				}
			}
		}
	}
	request.Metadata = nil
	return request, nil
}

func OpenAIToGeminiResponse(response llm.Response) (llm.Response, error) {
	if err := rejectMetadata(response.Metadata); err != nil {
		return llm.Response{}, err
	}
	if err := rejectMetadata(response.Usage.Metadata); err != nil {
		return llm.Response{}, err
	}
	if response.Usage.CacheWriteTokens != 0 {
		return llm.Response{}, unsupported("cache creation usage", "Gemini usage has no portable cache-write token field")
	}
	if response.StopReason == llm.StopReasonUnknown {
		return llm.Response{}, unsupported("finish_reason", "OpenAI finish reason has no Gemini finishReason mapping")
	}
	for _, block := range response.Content {
		switch block.(type) {
		case llm.DocumentBlock, llm.ImageBlock:
			return llm.Response{}, unsupported("response media", "Gemini response translation currently supports text and function calls only")
		case llm.ReasoningBlock:
			return llm.Response{}, unsupported("reasoning", "OpenAI reasoning content cannot yet be represented losslessly as Gemini thought parts")
		}
	}
	response.Metadata = nil
	response.Usage.Metadata = nil
	return response, nil
}

func GeminiToOpenAIResponse(response llm.Response) (llm.Response, error) {
	if err := rejectMetadata(response.Metadata); err != nil {
		return llm.Response{}, err
	}
	if err := rejectMetadata(response.Usage.Metadata); err != nil {
		return llm.Response{}, err
	}
	if response.Usage.CacheWriteTokens != 0 {
		return llm.Response{}, unsupported("cache creation usage", "Chat Completions has no cache-write token field")
	}
	if response.StopReason == llm.StopReasonUnknown {
		return llm.Response{}, unsupported("finishReason", "Gemini finish reason has no Chat Completions mapping")
	}
	for _, block := range response.Content {
		switch block.(type) {
		case llm.DocumentBlock, llm.ImageBlock:
			return llm.Response{}, unsupported("response media", "Chat Completions cannot represent Gemini media output")
		case llm.ReasoningBlock:
			return llm.Response{}, unsupported("thinking", "Gemini thought parts cannot yet be represented losslessly in Chat Completions")
		}
	}
	response.Metadata = nil
	response.Usage.Metadata = nil
	return response, nil
}

func checkOpenAIBlockForGemini(block llm.ContentBlock) error {
	switch value := block.(type) {
	case llm.ImageBlock:
		if value.Source.Type != llm.MediaSourceBase64 {
			return unsupported("image", "Gemini translation currently accepts only inline base64 images")
		}
	case llm.DocumentBlock:
		return unsupported("document", "OpenAI Chat document translation to Gemini is not implemented")
	case llm.ReasoningBlock:
		return unsupported("reasoning", "reasoning blocks cannot yet be translated to Gemini")
	case llm.ToolCallBlock:
		var object map[string]json.RawMessage
		if json.Unmarshal(value.Arguments, &object) != nil || object == nil {
			return unsupported("tool arguments", "Gemini function-call args must be a JSON object")
		}
	case llm.ToolResultBlock:
		if value.IsError {
			return unsupported("tool result error", "Gemini function response error semantics are not mapped yet")
		}
		if len(value.Content) != 1 {
			return unsupported("tool result", "Gemini function responses currently require one text block")
		}
		if _, ok := value.Content[0].(llm.TextBlock); !ok {
			return unsupported("tool result", "Gemini function responses currently require text content")
		}
	}
	return nil
}
