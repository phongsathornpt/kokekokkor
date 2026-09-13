package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func EncodeChatRequest(request llm.Request) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("validate canonical request: %w", err)
	}

	wire := chatRequest{
		Model:               request.Model,
		MaxCompletionTokens: request.MaxOutputTokens,
		Temperature:         request.Temperature,
		TopP:                request.TopP,
	}
	if request.Reasoning != nil {
		wire.ReasoningEffort = request.Reasoning.Effort
	}
	for _, message := range request.Messages {
		encoded, err := encodeChatMessage(message)
		if err != nil {
			return nil, err
		}
		wire.Messages = append(wire.Messages, encoded...)
	}
	for _, tool := range request.Tools {
		wire.Tools = append(wire.Tools, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  cloneRaw(tool.InputSchema),
			},
		})
	}

	var err error
	if wire.Stop, err = encodeChatStop(request.Stop); err != nil {
		return nil, err
	}
	if wire.ToolChoice, err = encodeChatToolChoice(request.ToolChoice); err != nil {
		return nil, err
	}
	if wire.ResponseFormat, err = encodeChatResponseFormat(request.ResponseFormat); err != nil {
		return nil, err
	}

	known, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode chat request: %w", err)
	}
	return mergeMetadata(known, request.Metadata)
}
