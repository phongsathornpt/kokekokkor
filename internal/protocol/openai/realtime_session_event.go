package openai

import (
	"encoding/json"
	"fmt"
)

func EncodeRealtimeSessionCreated(model string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":     "session.created",
		"event_id": "event_gateway_session_created",
		"session": map[string]any{
			"type":               "realtime",
			"object":             "realtime.session",
			"id":                 "sess_gateway",
			"model":              model,
			"output_modalities":  []string{"text"},
			"instructions":       "",
			"tools":              []any{},
			"tool_choice":        "auto",
			"max_output_tokens":  "inf",
			"tracing":            nil,
			"truncation":         "auto",
			"prompt":             nil,
			"audio":              nil,
			"include":            nil,
		},
	})
}

func EncodeRealtimeError(code, message string) ([]byte, error) {
	if code == "" {
		code = "translation_error"
	}
	if message == "" {
		message = "realtime translation failed"
	}
	data, err := json.Marshal(map[string]any{
		"type":     "error",
		"event_id": "event_gateway_error",
		"error": map[string]any{
			"type":    code,
			"code":    code,
			"message": message,
			"param":   nil,
			"event_id": nil,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI Realtime error: %w", err)
	}
	return data, nil
}
