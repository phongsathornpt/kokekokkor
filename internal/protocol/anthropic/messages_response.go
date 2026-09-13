package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type messagesResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	Model        string          `json:"model"`
	StopReason   string          `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        messagesUsage   `json:"usage"`
}

type messagesUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
}

func DecodeMessagesResponse(data []byte) (llm.Response, error) {
	var wire messagesResponse
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Response{}, fmt.Errorf("decode Anthropic Messages response: %w", err)
	}
	if wire.Role != "" && wire.Role != "assistant" {
		return llm.Response{}, fmt.Errorf("unexpected Anthropic response role %q", wire.Role)
	}
	content, err := decodeAnthropicContent(wire.Content)
	if err != nil {
		return llm.Response{}, fmt.Errorf("decode Anthropic response content: %w", err)
	}

	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(data, &metadata); err != nil {
		return llm.Response{}, fmt.Errorf("decode Anthropic response metadata: %w", err)
	}
	for _, key := range []string{"id", "type", "role", "content", "model", "stop_reason", "stop_sequence", "usage"} {
		delete(metadata, key)
	}

	response := llm.Response{
		ID:         wire.ID,
		Model:      wire.Model,
		Content:    content,
		StopReason: decodeAnthropicStopReason(wire.StopReason),
		Usage: llm.Usage{
			InputTokens:      wire.Usage.InputTokens,
			OutputTokens:     wire.Usage.OutputTokens,
			CacheReadTokens:  wire.Usage.CacheReadInputTokens,
			CacheWriteTokens: wire.Usage.CacheCreationInputTokens,
		},
		Metadata: metadata,
	}
	if wire.StopSequence != nil {
		response.StopSequence = *wire.StopSequence
	}
	return response, nil
}

func EncodeMessagesResponse(response llm.Response) ([]byte, error) {
	content, err := encodeAnthropicContent(response.Content)
	if err != nil {
		return nil, fmt.Errorf("encode Anthropic response content: %w", err)
	}
	stopReason, err := encodeAnthropicStopReason(response.StopReason)
	if err != nil {
		return nil, err
	}
	var stopSequence *string
	if response.StopSequence != "" {
		value := response.StopSequence
		stopSequence = &value
	}

	wire := messagesResponse{
		ID:           response.ID,
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		Model:        response.Model,
		StopReason:   stopReason,
		StopSequence: stopSequence,
		Usage: messagesUsage{
			InputTokens:              response.Usage.InputTokens,
			OutputTokens:             response.Usage.OutputTokens,
			CacheReadInputTokens:     response.Usage.CacheReadTokens,
			CacheCreationInputTokens: response.Usage.CacheWriteTokens,
		},
	}
	known, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode Anthropic Messages response: %w", err)
	}
	return mergeAnthropicMetadata(known, response.Metadata)
}

func decodeAnthropicStopReason(reason string) llm.StopReason {
	switch reason {
	case "end_turn":
		return llm.StopReasonEndTurn
	case "max_tokens":
		return llm.StopReasonMaxTokens
	case "stop_sequence":
		return llm.StopReasonStopSequence
	case "tool_use":
		return llm.StopReasonToolUse
	case "refusal":
		return llm.StopReasonContentBlock
	default:
		return llm.StopReasonUnknown
	}
}

func encodeAnthropicStopReason(reason llm.StopReason) (string, error) {
	switch reason {
	case llm.StopReasonEndTurn:
		return "end_turn", nil
	case llm.StopReasonMaxTokens:
		return "max_tokens", nil
	case llm.StopReasonStopSequence:
		return "stop_sequence", nil
	case llm.StopReasonToolUse:
		return "tool_use", nil
	case llm.StopReasonContentBlock:
		return "refusal", nil
	default:
		return "", fmt.Errorf("unsupported canonical stop reason %q for Anthropic response", reason)
	}
}
