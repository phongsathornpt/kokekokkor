package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func DecodeChatRequest(data []byte) (llm.Request, error) {
	var wire chatRequest
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Request{}, fmt.Errorf("decode chat request: %w", err)
	}

	metadata, err := chatMetadata(data)
	if err != nil {
		return llm.Request{}, err
	}
	request := llm.Request{
		Model:           wire.Model,
		Temperature:     wire.Temperature,
		TopP:            wire.TopP,
		MaxOutputTokens: wire.MaxCompletionTokens,
		Metadata:        metadata,
	}
	if request.MaxOutputTokens == nil {
		request.MaxOutputTokens = wire.MaxTokens
	}
	if wire.ReasoningEffort != "" {
		request.Reasoning = &llm.ReasoningConfig{Enabled: true, Effort: wire.ReasoningEffort}
	}
	if request.Stop, err = decodeChatStop(wire.Stop); err != nil {
		return llm.Request{}, err
	}
	if err := decodeChatMessages(&request, wire.Messages); err != nil {
		return llm.Request{}, err
	}
	decodeChatTools(&request, wire.Tools)
	if err := decodeChatOptions(&request, wire); err != nil {
		return llm.Request{}, err
	}
	if err := request.Validate(); err != nil {
		return llm.Request{}, fmt.Errorf("validate chat request: %w", err)
	}
	return request, nil
}
