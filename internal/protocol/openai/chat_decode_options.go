package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func chatMetadata(data []byte) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode chat metadata: %w", err)
	}
	for _, key := range []string{
		"model", "messages", "tools", "tool_choice", "parallel_tool_calls", "max_completion_tokens",
		"max_tokens", "temperature", "top_p", "stop", "response_format", "reasoning_effort",
	} {
		delete(raw, key)
	}
	return raw, nil
}

func decodeChatOptions(request *llm.Request, wire chatRequest) error {
	if len(wire.ToolChoice) != 0 && string(wire.ToolChoice) != "null" {
		choice, err := decodeChatToolChoice(wire.ToolChoice)
		if err != nil {
			return err
		}
		request.ToolChoice = choice
	}
	if wire.ParallelToolCalls != nil && !*wire.ParallelToolCalls {
		if request.ToolChoice == nil {
			request.ToolChoice = &llm.ToolChoice{Mode: llm.ToolChoiceAuto}
		}
		request.ToolChoice.DisableParallel = true
	}
	if len(wire.ResponseFormat) != 0 && string(wire.ResponseFormat) != "null" {
		format, err := decodeChatResponseFormat(wire.ResponseFormat)
		if err != nil {
			return err
		}
		request.ResponseFormat = format
	}
	return nil
}

func decodeChatTools(request *llm.Request, tools []chatTool) {
	for _, tool := range tools {
		if tool.Type != "" && tool.Type != "function" {
			continue
		}
		request.Tools = append(request.Tools, llm.Tool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: cloneRaw(tool.Function.Parameters),
		})
	}
}
