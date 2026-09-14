package openai

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeResponsesTextConfig(raw json.RawMessage) (*llm.ResponseFormat, json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, nil, fmt.Errorf("decode Responses text config: %w", err)
	}

	var responseFormat *llm.ResponseFormat
	if formatRaw, ok := object["format"]; ok {
		delete(object, "format")
		var format struct {
			Type        string          `json:"type"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Schema      json.RawMessage `json:"schema"`
			Strict      bool            `json:"strict"`
		}
		if err := json.Unmarshal(formatRaw, &format); err != nil {
			return nil, nil, fmt.Errorf("decode Responses text.format: %w", err)
		}
		switch format.Type {
		case "", "text":
		case "json_schema":
			responseFormat = &llm.ResponseFormat{
				Name:        format.Name,
				Description: format.Description,
				JSONSchema:  cloneRaw(format.Schema),
				Strict:      format.Strict,
			}
		default:
			object["format"] = cloneRaw(formatRaw)
		}
	}
	if len(object) == 0 {
		return responseFormat, nil, nil
	}
	extras, err := json.Marshal(object)
	if err != nil {
		return nil, nil, err
	}
	return responseFormat, extras, nil
}
