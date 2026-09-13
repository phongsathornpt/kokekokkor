package translation

import (
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func ResponsesToAnthropicRequest(request llm.Request) (llm.Request, error) {
	return OpenAIToAnthropicRequest(request)
}

func AnthropicToResponsesResponse(response llm.Response) (llm.Response, error) {
	if err := rejectMetadata(response.Metadata); err != nil {
		return llm.Response{}, err
	}
	if err := rejectMetadata(response.Usage.Metadata); err != nil {
		return llm.Response{}, err
	}
	if response.StopReason == llm.StopReasonUnknown {
		return llm.Response{}, unsupported("stop_reason", "upstream stop reason has no Responses mapping")
	}
	for _, block := range response.Content {
		switch block.(type) {
		case llm.TextBlock, llm.ToolCallBlock:
		case llm.ReasoningBlock:
			return llm.Response{}, unsupported("thinking", "reasoning blocks cannot yet be represented losslessly in Responses output")
		default:
			return llm.Response{}, unsupported("response content", fmt.Sprintf("block %T cannot be represented in Responses output", block))
		}
	}
	response.Metadata = nil
	response.Usage.Metadata = nil
	return response, nil
}
