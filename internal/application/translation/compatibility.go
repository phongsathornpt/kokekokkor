package translation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

var ErrUnsupported = errors.New("unsupported cross-protocol feature")

type CompatibilityError struct {
	Feature string
	Reason  string
}

func (e CompatibilityError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("%s: %s", ErrUnsupported, e.Feature)
	}
	return fmt.Sprintf("%s: %s: %s", ErrUnsupported, e.Feature, e.Reason)
}

func (e CompatibilityError) Unwrap() error { return ErrUnsupported }

// ResponseError marks a failure that happened after an upstream request
// succeeded. Callers must not fall back to another provider because doing so
// could duplicate a completed generation.
type ResponseError struct {
	Err error
}

func (e ResponseError) Error() string { return "translate upstream response: " + e.Err.Error() }
func (e ResponseError) Unwrap() error { return e.Err }

func WrapResponse(err error) error {
	if err == nil {
		return nil
	}
	return ResponseError{Err: err}
}

func OpenAIToAnthropicRequest(request llm.Request) (llm.Request, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := consumeStreaming(metadata); err != nil {
		return llm.Request{}, err
	}
	if raw, ok := metadata["n"]; ok {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil || n != 1 {
			return llm.Request{}, unsupported("n", "Anthropic Messages supports one generated message per request")
		}
		delete(metadata, "n")
	}
	if err := rejectMetadata(metadata); err != nil {
		return llm.Request{}, err
	}
	if request.MaxOutputTokens == nil {
		return llm.Request{}, unsupported("max_tokens", "Anthropic Messages requires an explicit output-token limit")
	}
	reasoning, err := openAIReasoningToAnthropic(request.Reasoning)
	if err != nil {
		return llm.Request{}, err
	}
	request.Reasoning = reasoning
	if err := rejectNestedMetadata(request); err != nil {
		return llm.Request{}, err
	}

	request.Metadata = nil
	return request, nil
}

func AnthropicToOpenAIRequest(request llm.Request) (llm.Request, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := consumeStreaming(metadata); err != nil {
		return llm.Request{}, err
	}
	if err := rejectMetadata(metadata); err != nil {
		return llm.Request{}, err
	}
	if request.Reasoning != nil {
		return llm.Request{}, unsupported("thinking", "Anthropic thinking controls cannot yet be represented losslessly in Chat Completions")
	}
	if request.ToolChoice != nil && request.ToolChoice.DisableParallel {
		return llm.Request{}, unsupported("disable_parallel_tool_use", "parallel-tool policy translation is not implemented yet")
	}
	if err := rejectNestedMetadata(request); err != nil {
		return llm.Request{}, err
	}
	for _, message := range request.Messages {
		for _, block := range message.Content {
			if err := checkAnthropicBlockForOpenAI(block); err != nil {
				return llm.Request{}, err
			}
		}
	}

	request.Metadata = nil
	return request, nil
}

func OpenAIToAnthropicResponse(response llm.Response) (llm.Response, error) {
	if err := rejectMetadata(response.Metadata); err != nil {
		return llm.Response{}, err
	}
	if err := rejectMetadata(response.Usage.Metadata); err != nil {
		return llm.Response{}, err
	}
	if response.Usage.ReasoningTokens != 0 {
		return llm.Response{}, unsupported("reasoning usage", "Anthropic usage has no portable reasoning-token field")
	}
	if response.StopReason == llm.StopReasonUnknown {
		return llm.Response{}, unsupported("finish_reason", "upstream finish reason has no Anthropic stop_reason mapping")
	}
	response.Metadata = nil
	response.Usage.Metadata = nil
	return response, nil
}

func AnthropicToOpenAIResponse(response llm.Response) (llm.Response, error) {
	if err := rejectMetadata(response.Metadata); err != nil {
		return llm.Response{}, err
	}
	if err := rejectMetadata(response.Usage.Metadata); err != nil {
		return llm.Response{}, err
	}
	if response.Usage.CacheWriteTokens != 0 {
		return llm.Response{}, unsupported("cache creation usage", "Chat Completions usage has no cache-write token field")
	}
	if response.StopReason == llm.StopReasonUnknown {
		return llm.Response{}, unsupported("stop_reason", "upstream stop reason has no Chat Completions finish_reason mapping")
	}
	response.Content = stripReasoningBlocks(response.Content)
	response.Metadata = nil
	response.Usage.Metadata = nil
	return response, nil
}

func consumeStreaming(metadata map[string]json.RawMessage) error {
	raw, ok := metadata["stream"]
	if !ok {
		return nil
	}
	var stream bool
	if err := json.Unmarshal(raw, &stream); err != nil {
		return unsupported("stream", "stream must be a boolean")
	}
	if stream {
		return unsupported("stream", "cross-protocol streaming is implemented in a later slice")
	}
	delete(metadata, "stream")
	return nil
}

func rejectNestedMetadata(request llm.Request) error {
	for i, message := range request.Messages {
		if len(message.Metadata) != 0 {
			return unsupported("message metadata", fmt.Sprintf("message %d contains provider-specific metadata", i))
		}
	}
	for i, tool := range request.Tools {
		if len(tool.Metadata) != 0 {
			return unsupported("tool metadata", fmt.Sprintf("tool %d contains provider-specific metadata", i))
		}
	}
	return nil
}

func checkAnthropicBlockForOpenAI(block llm.ContentBlock) error {
	switch value := block.(type) {
	case llm.DocumentBlock:
		return unsupported("document", "Chat Completions document-block translation is not implemented")
	case llm.ReasoningBlock:
		return unsupported("thinking", "thinking blocks cannot yet be represented losslessly in Chat Completions")
	case llm.ImageBlock:
		if value.Source.Type == llm.MediaSourceFile {
			return unsupported("file image", "Chat Completions file-backed image translation is not implemented")
		}
	case llm.ToolResultBlock:
		if value.IsError {
			return unsupported("tool_result.is_error", "Chat Completions tool messages have no equivalent error flag")
		}
		for _, nested := range value.Content {
			if _, ok := nested.(llm.TextBlock); !ok {
				return unsupported("tool_result content", "Chat Completions tool-result translation currently supports text only")
			}
		}
	}
	return nil
}

func rejectMetadata(metadata map[string]json.RawMessage) error {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return unsupported("provider extensions", "unsupported fields: "+strings.Join(keys, ", "))
}

func cloneMetadata(metadata map[string]json.RawMessage) map[string]json.RawMessage {
	if len(metadata) == 0 {
		return nil
	}
	clone := make(map[string]json.RawMessage, len(metadata))
	for key, value := range metadata {
		clone[key] = append(json.RawMessage(nil), value...)
	}
	return clone
}

func unsupported(feature, reason string) error {
	return CompatibilityError{Feature: feature, Reason: reason}
}
