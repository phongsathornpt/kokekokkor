package translation

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func OpenAIToGeminiStreamRequest(request llm.Request) (llm.Request, OpenAIStreamOptions, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := requireStreaming(metadata); err != nil {
		return llm.Request{}, OpenAIStreamOptions{}, err
	}
	options, err := consumeOpenAIStreamOptions(metadata)
	if err != nil {
		return llm.Request{}, OpenAIStreamOptions{}, err
	}
	metadata["stream"] = json.RawMessage("false")
	request.Metadata = metadata
	request, err = OpenAIToGeminiRequest(request)
	if err != nil {
		return llm.Request{}, OpenAIStreamOptions{}, err
	}
	return request, options, nil
}

func ResponsesToGeminiStreamRequest(request llm.Request) (llm.Request, ResponsesStreamOptions, error) {
	for _, tool := range request.Tools {
		if tool.Kind == llm.ToolKindWebSearch {
			return llm.Request{}, ResponsesStreamOptions{}, unsupported("web_search streaming", "Gemini grounding search calls and citations are not yet losslessly represented in translated Responses streams")
		}
	}
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
	request, err = ResponsesToGeminiRequest(request)
	if err != nil {
		return llm.Request{}, ResponsesStreamOptions{}, err
	}
	return request, options, nil
}

func GeminiToOpenAIStreamRequest(request llm.Request) (llm.Request, error) {
	request, err := GeminiToOpenAIRequest(request)
	if err != nil {
		return llm.Request{}, err
	}
	request.Metadata = map[string]json.RawMessage{
		"stream":         json.RawMessage("true"),
		"stream_options": json.RawMessage(`{"include_usage":true}`),
	}
	return request, nil
}

func GeminiToOpenAIStreamEvent(event llm.StreamEvent) error {
	return validateGeminiStreamEvent(event, true, false)
}

func GeminiToResponsesStreamEvent(event llm.StreamEvent) error {
	return geminiToResponsesStreamEvent(event, false)
}

func GeminiToResponsesBufferedStreamEvent(event llm.StreamEvent) error {
	return geminiToResponsesStreamEvent(event, true)
}

func geminiToResponsesStreamEvent(event llm.StreamEvent, allowRefusal bool) error {
	if err := validateGeminiStreamEvent(event, true, true); err != nil {
		return err
	}
	if event.Type == llm.StreamEventResponseStop && event.StopReason == llm.StopReasonContentBlock && !allowRefusal {
		return unsupported("refusal", "Gemini safety/content-block stop requires stream_options.buffer_refusals=true")
	}
	return nil
}

func OpenAIToGeminiStreamEvent(event llm.StreamEvent) error {
	if len(event.Metadata) != 0 {
		return unsupported("stream event metadata", fmt.Sprintf("event %q contains provider-specific metadata", event.Type))
	}
	if event.Usage != nil {
		if len(event.Usage.Metadata) != 0 {
			return unsupported("usage metadata", "usage contains provider-specific metadata")
		}
		if event.Usage.CacheWriteTokens != 0 {
			return unsupported("cache creation usage", "Gemini usage has no portable cache-write token field")
		}
	}
	switch event.Type {
	case llm.StreamEventReasoningDelta:
		return unsupported("reasoning stream", "OpenAI reasoning cannot yet be represented as Gemini thought parts")
	case llm.StreamEventError:
		return unsupported("stream error", "OpenAI stream errors are not mapped to Gemini stream chunks")
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
			return unsupported("finish_reason", "OpenAI finish reason has no Gemini finishReason mapping")
		}
	}
	return nil
}

func validateGeminiStreamEvent(event llm.StreamEvent, allowContentBlock, allowReasoning bool) error {
	if len(event.Metadata) != 0 {
		return unsupported("stream event metadata", fmt.Sprintf("event %q contains provider-specific Gemini metadata", event.Type))
	}
	if event.Usage != nil {
		if len(event.Usage.Metadata) != 0 {
			return unsupported("usage metadata", "Gemini usage contains provider-specific metadata")
		}
		if event.Usage.CacheWriteTokens != 0 {
			return unsupported("cache creation usage", "Gemini usage has no portable cache-write token field")
		}
	}
	switch event.Type {
	case llm.StreamEventReasoningDelta:
		if !allowReasoning {
			return unsupported("thinking stream", "Gemini thought parts cannot yet be represented losslessly")
		}
	case llm.StreamEventError:
		return unsupported("stream error", "Gemini stream errors are not translated into downstream terminal events")
	case llm.StreamEventContentStart:
		switch block := event.Block.(type) {
		case llm.TextBlock, llm.ToolCallBlock:
		case llm.ReasoningBlock:
			if !allowReasoning {
				return unsupported("thinking stream", "Gemini thought parts cannot yet be represented losslessly")
			}
			if block.Signature != "" || block.RedactedData != "" {
				return unsupported("reasoning state", "provider state cannot be represented downstream")
			}
		default:
			return unsupported("response content", fmt.Sprintf("stream block %T cannot be represented downstream", event.Block))
		}
	case llm.StreamEventResponseStop:
		switch event.StopReason {
		case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse, llm.StopReasonMaxTokens:
		case llm.StopReasonContentBlock:
			if !allowContentBlock {
				return unsupported("content block", "Gemini content-block stops are not supported by this downstream protocol")
			}
		default:
			return unsupported("finishReason", "Gemini finish reason has no downstream mapping")
		}
	}
	return nil
}
