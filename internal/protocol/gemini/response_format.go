package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeGeminiResponseFormat(raw json.RawMessage) (*llm.ResponseFormat, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, fmt.Errorf("decode Gemini responseFormat: %w", err)
	}
	textRaw, ok := object["text"]
	if !ok || len(object) != 1 {
		return nil, fmt.Errorf("Gemini responseFormat translation supports text output only")
	}
	var text struct {
		MIMEType string          `json:"mimeType"`
		Schema   json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(textRaw, &text); err != nil {
		return nil, fmt.Errorf("decode Gemini responseFormat.text: %w", err)
	}
	if text.MIMEType != "APPLICATION_JSON" && text.MIMEType != "application/json" {
		return nil, fmt.Errorf("unsupported Gemini responseFormat text mimeType %q", text.MIMEType)
	}
	if len(text.Schema) == 0 || !json.Valid(text.Schema) {
		return nil, fmt.Errorf("Gemini JSON responseFormat requires a valid schema")
	}
	return &llm.ResponseFormat{JSONSchema: append(json.RawMessage(nil), text.Schema...), Strict: true}, nil
}

func encodeGeminiResponseFormat(format *llm.ResponseFormat) (json.RawMessage, error) {
	if format == nil {
		return nil, nil
	}
	if len(format.JSONSchema) == 0 || !json.Valid(format.JSONSchema) {
		return nil, fmt.Errorf("Gemini structured output requires a valid JSON schema")
	}
	return json.Marshal(map[string]any{
		"text": map[string]any{
			"mimeType": "APPLICATION_JSON",
			"schema":   json.RawMessage(format.JSONSchema),
		},
	})
}
