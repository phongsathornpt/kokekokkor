package translation

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type ResponsesStreamOptions struct {
	IncludeObfuscation bool
}

func ResponsesToAnthropicStreamRequest(request llm.Request) (llm.Request, ResponsesStreamOptions, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := requireStreaming(metadata); err != nil {
		return llm.Request{}, ResponsesStreamOptions{}, err
	}
	options, err := consumeResponsesStreamOptions(metadata)
	if err != nil {
		return llm.Request{}, ResponsesStreamOptions{}, err
	}

	metadata["stream"] = json.RawMessage("false")
	request.Metadata = metadata
	request, err = ResponsesToAnthropicRequest(request)
	if err != nil {
		return llm.Request{}, ResponsesStreamOptions{}, err
	}
	request.Metadata = map[string]json.RawMessage{"stream": json.RawMessage("true")}
	return request, options, nil
}

func AnthropicToResponsesStreamEvent(event llm.StreamEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("stream event metadata", fmt.Sprintf("event %q contains provider-specific metadata", event.Type))
	}
	if event.Type == llm.StreamEventError {
		return unsupported("stream error", "provider stream errors are not translated into Responses terminal events yet")
	}
	if event.Usage != nil && len(event.Usage.Metadata) != 0 {
		return unsupported("usage metadata", "usage contains provider-specific metadata")
	}
	if event.Type == llm.StreamEventContentStart {
		switch block := event.Block.(type) {
		case llm.TextBlock, llm.ToolCallBlock:
		case llm.ReasoningBlock:
			if block.Signature != "" || block.RedactedData != "" {
				return unsupported("reasoning state", "provider state cannot be represented by Responses")
			}
		default:
			return unsupported("response content", fmt.Sprintf("stream block %T cannot be represented in Responses output", event.Block))
		}
	}
	if event.Type == llm.StreamEventResponseStop {
		switch event.StopReason {
		case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse, llm.StopReasonMaxTokens:
		case llm.StopReasonContentBlock:
			return unsupported("refusal", "Anthropic refusal/content-block stop is not mapped to Responses refusal events yet")
		default:
			return unsupported("stop_reason", "upstream stop reason has no Responses terminal-event mapping")
		}
	}
	return nil
}

func consumeResponsesStreamOptions(metadata map[string]json.RawMessage) (ResponsesStreamOptions, error) {
	options := ResponsesStreamOptions{IncludeObfuscation: true}
	raw, ok := metadata["stream_options"]
	if !ok {
		return options, nil
	}
	delete(metadata, "stream_options")

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return ResponsesStreamOptions{}, unsupported("stream_options", "must be an object")
	}
	for key, value := range object {
		switch key {
		case "include_obfuscation":
			if err := json.Unmarshal(value, &options.IncludeObfuscation); err != nil {
				return ResponsesStreamOptions{}, unsupported("stream_options.include_obfuscation", "must be a boolean")
			}
		default:
			return ResponsesStreamOptions{}, unsupported("stream_options", fmt.Sprintf("unsupported field %q", key))
		}
	}
	return options, nil
}
