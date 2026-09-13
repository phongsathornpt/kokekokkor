package openai

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeChatResponseFormat(raw json.RawMessage) (*llm.ResponseFormat, error) {
	var format struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Schema      json.RawMessage `json:"schema"`
			Strict      bool            `json:"strict"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(raw, &format); err != nil {
		return nil, fmt.Errorf("decode response_format: %w", err)
	}
	if format.Type != "json_schema" {
		return nil, nil
	}
	return &llm.ResponseFormat{
		Name:        format.JSONSchema.Name,
		Description: format.JSONSchema.Description,
		JSONSchema:  cloneRaw(format.JSONSchema.Schema),
		Strict:      format.JSONSchema.Strict,
	}, nil
}

func encodeChatResponseFormat(format *llm.ResponseFormat) (json.RawMessage, error) {
	if format == nil {
		return nil, nil
	}
	return json.Marshal(map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":        format.Name,
			"description": format.Description,
			"schema":      json.RawMessage(format.JSONSchema),
			"strict":      format.Strict,
		},
	})
}
