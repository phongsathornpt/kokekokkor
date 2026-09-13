package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func flattenText(blocks []llm.ContentBlock) (string, error) {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		text, ok := block.(llm.TextBlock)
		if !ok {
			return "", fmt.Errorf("chat tool results currently support text content only")
		}
		parts = append(parts, text.Text)
	}
	return strings.Join(parts, "\n"), nil
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func mergeMetadata(known []byte, metadata map[string]json.RawMessage) ([]byte, error) {
	if len(metadata) == 0 {
		return known, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(known, &object); err != nil {
		return nil, err
	}
	for key, value := range metadata {
		if _, exists := object[key]; exists {
			continue
		}
		object[key] = cloneRaw(value)
	}
	return json.Marshal(object)
}
