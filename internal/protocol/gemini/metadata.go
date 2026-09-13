package gemini

import (
	"encoding/json"
	"fmt"
)

func topLevelMetadata(data []byte, known ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("decode Gemini request object: %w", err)
	}
	for _, key := range known {
		delete(object, key)
	}
	if len(object) == 0 {
		return nil, nil
	}
	return object, nil
}

func nestedExtras(data []byte, field string, known ...string) json.RawMessage {
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil {
		return nil
	}
	raw, ok := object[field]
	if !ok {
		return nil
	}
	var nested map[string]json.RawMessage
	if json.Unmarshal(raw, &nested) != nil {
		return raw
	}
	for _, key := range known {
		delete(nested, key)
	}
	if len(nested) == 0 {
		return nil
	}
	encoded, _ := json.Marshal(nested)
	return encoded
}

func putMetadata(metadata map[string]json.RawMessage, key string, value json.RawMessage) map[string]json.RawMessage {
	if len(value) == 0 {
		return metadata
	}
	if metadata == nil {
		metadata = make(map[string]json.RawMessage)
	}
	metadata[key] = append(json.RawMessage(nil), value...)
	return metadata
}
