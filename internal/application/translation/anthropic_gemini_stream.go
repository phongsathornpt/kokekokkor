package translation

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func AnthropicToGeminiStreamRequest(request llm.Request) (llm.Request, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := requireStreaming(metadata); err != nil {
		return llm.Request{}, err
	}
	metadata["stream"] = json.RawMessage("false")
	request.Metadata = metadata
	return AnthropicToGeminiRequest(request)
}

func GeminiToAnthropicStreamRequest(request llm.Request) (llm.Request, error) {
	request, err := GeminiToAnthropicRequest(request)
	if err != nil {
		return llm.Request{}, err
	}
	if request.MaxOutputTokens == nil {
		return llm.Request{}, unsupported("max output tokens", "Anthropic Messages requires max_tokens for translated Gemini requests")
	}
	request.Metadata = map[string]json.RawMessage{"stream": json.RawMessage("true")}
	return request, nil
}

func AnthropicToGeminiStreamEvent(event llm.StreamEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("stream event metadata", fmt.Sprintf("event %q contains provider-specific Anthropic metadata", event.Type))
	}
	if event.Usage != nil {
		if len(event.Usage.Metadata) != 0 {
			return unsupported("usage metadata", "Anthropic usage contains provider-specific metadata")
		}
		if event.Usage.CacheWriteTokens != 0 {
			return unsupported("cache creation usage", "Gemini usage has no portable cache-write token field")
		}
	}
	switch event.Type {
	case llm.StreamEventReasoningDelta:
		return unsupported("thinking stream", "Anthropic thinking cannot yet be represented as Gemini thought parts")
	case llm.StreamEventError:
		return unsupported("stream error", "Anthropic stream errors are not mapped to Gemini stream chunks")
	case llm.StreamEventContentStart:
		switch event.Block.(type) {
		case llm.TextBlock, llm.ToolCallBlock:
		default:
			return unsupported("response content", fmt.Sprintf("stream block %T cannot be represented in Gemini output", event.Block))
		}
	case llm.StreamEventResponseStop:
		switch event.StopReason {
		case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse, llm.StopReasonMaxTokens, llm.StopReasonContentBlock:
		default:
			return unsupported("stop reason", "Anthropic stop reason has no Gemini finishReason mapping")
		}
	}
	return nil
}

func GeminiToAnthropicStreamEvent(event llm.StreamEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("stream event metadata", fmt.Sprintf("event %q contains provider-specific Gemini metadata", event.Type))
	}
	if event.Usage != nil {
		if len(event.Usage.Metadata) != 0 {
			return unsupported("usage metadata", "Gemini usage contains provider-specific metadata")
		}
		if event.Usage.ReasoningTokens != 0 {
			return unsupported("reasoning usage", "Anthropic Messages usage has no portable reasoning-token field")
		}
	}
	switch event.Type {
	case llm.StreamEventReasoningDelta:
		return unsupported("thinking stream", "Gemini thought parts cannot yet be represented in Anthropic Messages")
	case llm.StreamEventError:
		return unsupported("stream error", "Gemini stream errors are not mapped to Anthropic terminal events")
	case llm.StreamEventContentStart:
		switch event.Block.(type) {
		case llm.TextBlock, llm.ToolCallBlock:
		default:
			return unsupported("response content", fmt.Sprintf("stream block %T cannot be represented in Anthropic output", event.Block))
		}
	case llm.StreamEventResponseStop:
		switch event.StopReason {
		case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse, llm.StopReasonMaxTokens:
		case llm.StopReasonContentBlock:
			return unsupported("content block", "Gemini safety/content-block stops have no Anthropic Messages stop_reason mapping")
		default:
			return unsupported("finishReason", "Gemini finish reason has no Anthropic stop_reason mapping")
		}
	}
	return nil
}
