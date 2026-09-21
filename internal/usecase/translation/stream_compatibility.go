package translation

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type OpenAIStreamOptions struct {
	IncludeUsage bool
}

func OpenAIToAnthropicStreamRequest(request llm.Request) (llm.Request, OpenAIStreamOptions, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := requireStreaming(metadata); err != nil {
		return llm.Request{}, OpenAIStreamOptions{}, err
	}
	options, err := consumeOpenAIStreamOptions(metadata)
	if err != nil {
		return llm.Request{}, OpenAIStreamOptions{}, err
	}

	// Reuse the non-stream compatibility checks after consuming stream-only
	// controls. The resulting Anthropic wire request explicitly re-enables
	// streaming below.
	encodedFalse, _ := json.Marshal(false)
	metadata["stream"] = encodedFalse
	request.Metadata = metadata
	request, err = OpenAIToAnthropicRequest(request)
	if err != nil {
		return llm.Request{}, OpenAIStreamOptions{}, err
	}
	request.Metadata = map[string]json.RawMessage{"stream": json.RawMessage("true")}
	return request, options, nil
}

func AnthropicToOpenAIStreamRequest(request llm.Request) (llm.Request, error) {
	metadata := cloneMetadata(request.Metadata)
	if err := requireStreaming(metadata); err != nil {
		return llm.Request{}, err
	}
	delete(metadata, "stream")
	request.Metadata = metadata

	request, err := AnthropicToOpenAIRequest(request)
	if err != nil {
		return llm.Request{}, err
	}
	request.Metadata = map[string]json.RawMessage{
		"stream":         json.RawMessage("true"),
		"stream_options": json.RawMessage(`{"include_usage":true}`),
	}
	return request, nil
}

func requireStreaming(metadata map[string]json.RawMessage) error {
	raw, ok := metadata["stream"]
	if !ok {
		return unsupported("stream", "streaming translation requires stream: true")
	}
	var stream bool
	if err := json.Unmarshal(raw, &stream); err != nil {
		return unsupported("stream", "stream must be a boolean")
	}
	if !stream {
		return unsupported("stream", "streaming translation requires stream: true")
	}
	return nil
}

func consumeOpenAIStreamOptions(metadata map[string]json.RawMessage) (OpenAIStreamOptions, error) {
	raw, ok := metadata["stream_options"]
	if !ok {
		return OpenAIStreamOptions{}, nil
	}
	delete(metadata, "stream_options")

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return OpenAIStreamOptions{}, unsupported("stream_options", "must be an object")
	}
	var options OpenAIStreamOptions
	for key, value := range object {
		switch key {
		case "include_usage":
			if err := json.Unmarshal(value, &options.IncludeUsage); err != nil {
				return OpenAIStreamOptions{}, unsupported("stream_options.include_usage", "must be a boolean")
			}
		default:
			return OpenAIStreamOptions{}, unsupported("stream_options", fmt.Sprintf("unsupported field %q", key))
		}
	}
	return options, nil
}
