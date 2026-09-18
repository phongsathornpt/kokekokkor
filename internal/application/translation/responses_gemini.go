package translation

import (
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func ResponsesToGeminiRequest(request llm.Request) (llm.Request, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := consumeResponsesStatelessFlags(metadata); err != nil {
		return llm.Request{}, err
	}
	if err := consumeStreaming(metadata); err != nil {
		return llm.Request{}, err
	}
	if err := rejectMetadata(metadata); err != nil {
		return llm.Request{}, err
	}
	reasoning, err := openAIReasoningToGemini(request.Reasoning)
	if err != nil {
		return llm.Request{}, err
	}
	request.Reasoning = reasoning
	hasWebSearch := false
	for _, tool := range request.Tools {
		if tool.Kind == llm.ToolKindWebSearch {
			hasWebSearch = true
		}
	}
	if hasWebSearch && request.ToolChoice != nil {
		if request.ToolChoice.Mode != llm.ToolChoiceAuto || request.ToolChoice.DisableParallel {
			return llm.Request{}, unsupported("tool_choice", "Gemini Google Search cannot preserve forced, disabled, named, or parallel web-search selection")
		}
		request.ToolChoice = nil
	}
	if request.ToolChoice != nil && request.ToolChoice.DisableParallel {
		return llm.Request{}, unsupported("parallel_tool_calls", "Gemini parallel-tool policy translation is not implemented")
	}
	if err := rejectNestedMetadata(request); err != nil {
		return llm.Request{}, err
	}

	normalized := make([]llm.Message, 0, len(request.Messages))
	var instructions []llm.ContentBlock
	var instructionRole llm.Role
	seenConversation := false
	for i, message := range request.Messages {
		if !seenConversation && (message.Role == llm.RoleSystem || message.Role == llm.RoleDeveloper) {
			if instructionRole != "" && instructionRole != message.Role {
				return llm.Request{}, unsupported("instruction roles", "Gemini systemInstruction cannot preserve distinct system and developer precedence")
			}
			instructionRole = message.Role
			for _, block := range message.Content {
				if _, ok := block.(llm.TextBlock); !ok {
					return llm.Request{}, unsupported("instructions", fmt.Sprintf("instruction message %d contains non-text content", i))
				}
				instructions = append(instructions, block)
			}
			continue
		}
		seenConversation = true
		if message.Role == llm.RoleSystem || message.Role == llm.RoleDeveloper {
			return llm.Request{}, unsupported("instruction order", "Gemini systemInstruction cannot preserve interleaved system or developer messages")
		}
		for _, block := range message.Content {
			if err := checkOpenAIBlockForGemini(block); err != nil {
				return llm.Request{}, err
			}
		}
		normalized = append(normalized, message)
	}
	if len(instructions) != 0 {
		normalized = append([]llm.Message{{Role: llm.RoleSystem, Content: instructions}}, normalized...)
	}

	request.Messages = normalized
	request.Metadata = nil
	return request, nil
}

func GeminiToResponsesResponse(response llm.Response) (llm.Response, error) {
	if err := rejectMetadata(response.Metadata); err != nil {
		return llm.Response{}, err
	}
	if err := rejectMetadata(response.Usage.Metadata); err != nil {
		return llm.Response{}, err
	}
	if response.Usage.CacheWriteTokens != 0 {
		return llm.Response{}, unsupported("cache creation usage", "Responses has no portable cache-write token field")
	}
	if response.StopReason == llm.StopReasonUnknown {
		return llm.Response{}, unsupported("finishReason", "Gemini finish reason has no Responses mapping")
	}
	for _, block := range response.Content {
		switch block.(type) {
		case llm.TextBlock, llm.ToolCallBlock, llm.ReasoningBlock, llm.WebSearchCallBlock:
		default:
			return llm.Response{}, unsupported("response content", fmt.Sprintf("Gemini block %T cannot be represented in Responses output", block))
		}
	}
	content, err := reasoningSummariesForResponses(response.Content)
	if err != nil {
		return llm.Response{}, err
	}
	response.Content = content
	response.Metadata = nil
	response.Usage.Metadata = nil
	return response, nil
}
