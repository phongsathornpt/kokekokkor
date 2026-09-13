package openai

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type chatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
	Usage   *chatCompletionUsage   `json:"usage,omitempty"`
}

type chatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatCompletionUsage struct {
	PromptTokens            int64                   `json:"prompt_tokens"`
	CompletionTokens        int64                   `json:"completion_tokens"`
	TotalTokens             int64                   `json:"total_tokens"`
	PromptTokensDetails     *promptTokenDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *completionTokenDetails `json:"completion_tokens_details,omitempty"`
}

type promptTokenDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

type completionTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

func DecodeChatResponse(data []byte) (llm.Response, error) {
	var wire chatCompletionResponse
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Response{}, fmt.Errorf("decode OpenAI chat response: %w", err)
	}
	if len(wire.Choices) != 1 {
		return llm.Response{}, fmt.Errorf("OpenAI chat response has %d choices; cross-protocol translation requires exactly one", len(wire.Choices))
	}

	messages, err := decodeChatMessage(wire.Choices[0].Message)
	if err != nil {
		return llm.Response{}, fmt.Errorf("decode OpenAI assistant message: %w", err)
	}
	if len(messages) != 1 || messages[0].Role != llm.RoleAssistant {
		return llm.Response{}, fmt.Errorf("OpenAI chat response did not contain one assistant message")
	}

	response := llm.Response{
		ID:         wire.ID,
		Model:      wire.Model,
		Content:    messages[0].Content,
		StopReason: decodeChatFinishReason(wire.Choices[0].FinishReason),
	}
	if wire.Usage != nil {
		response.Usage.InputTokens = wire.Usage.PromptTokens
		response.Usage.OutputTokens = wire.Usage.CompletionTokens
		if wire.Usage.PromptTokensDetails != nil {
			response.Usage.CacheReadTokens = wire.Usage.PromptTokensDetails.CachedTokens
		}
		if wire.Usage.CompletionTokensDetails != nil {
			response.Usage.ReasoningTokens = wire.Usage.CompletionTokensDetails.ReasoningTokens
		}
	}
	return response, nil
}

func EncodeChatResponse(response llm.Response) ([]byte, error) {
	messages, err := encodeChatMessage(llm.Message{Role: llm.RoleAssistant, Content: response.Content})
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI assistant message: %w", err)
	}
	if len(messages) != 1 || messages[0].Role != "assistant" {
		return nil, fmt.Errorf("canonical response cannot be represented as one OpenAI assistant message")
	}
	finishReason, err := encodeChatFinishReason(response.StopReason)
	if err != nil {
		return nil, err
	}

	usage := &chatCompletionUsage{
		PromptTokens:     response.Usage.InputTokens,
		CompletionTokens: response.Usage.OutputTokens,
		TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
	}
	if response.Usage.CacheReadTokens != 0 {
		usage.PromptTokensDetails = &promptTokenDetails{CachedTokens: response.Usage.CacheReadTokens}
	}
	if response.Usage.ReasoningTokens != 0 {
		usage.CompletionTokensDetails = &completionTokenDetails{ReasoningTokens: response.Usage.ReasoningTokens}
	}

	wire := chatCompletionResponse{
		ID:      response.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   response.Model,
		Choices: []chatCompletionChoice{{
			Index:        0,
			Message:      messages[0],
			FinishReason: finishReason,
		}},
		Usage: usage,
	}
	known, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI chat response: %w", err)
	}
	return mergeMetadata(known, response.Metadata)
}

func decodeChatFinishReason(reason string) llm.StopReason {
	switch reason {
	case "stop":
		return llm.StopReasonEndTurn
	case "length":
		return llm.StopReasonMaxTokens
	case "tool_calls", "function_call":
		return llm.StopReasonToolUse
	case "content_filter":
		return llm.StopReasonContentBlock
	default:
		return llm.StopReasonUnknown
	}
}

func encodeChatFinishReason(reason llm.StopReason) (string, error) {
	switch reason {
	case llm.StopReasonEndTurn, llm.StopReasonStopSequence:
		return "stop", nil
	case llm.StopReasonMaxTokens:
		return "length", nil
	case llm.StopReasonToolUse:
		return "tool_calls", nil
	case llm.StopReasonContentBlock:
		return "content_filter", nil
	default:
		return "", fmt.Errorf("unsupported canonical stop reason %q for OpenAI chat response", reason)
	}
}
