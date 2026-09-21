package openai

import (
	"encoding/json"
	"time"
)

type BackgroundResponseState struct {
	ID                 string
	Model              string
	Status             string
	PreviousResponseID string
	ConversationID     string
	Store              bool
	CreatedAt          int64
	ErrorMessage       string
}

func EncodeBackgroundResponseState(state BackgroundResponseState) ([]byte, error) {
	if state.CreatedAt == 0 {
		state.CreatedAt = time.Now().Unix()
	}
	var conversation any
	if state.ConversationID != "" {
		conversation = responseConversation{ID: state.ConversationID}
	}
	var responseError any
	if state.ErrorMessage != "" {
		responseError = map[string]any{
			"code":    "background_error",
			"message": state.ErrorMessage,
		}
	}
	return json.Marshal(map[string]any{
		"id":                   NormalizeResponsesID(state.ID),
		"object":               "response",
		"created_at":           state.CreatedAt,
		"status":               state.Status,
		"error":                responseError,
		"incomplete_details":   nil,
		"model":                state.Model,
		"output":               []any{},
		"parallel_tool_calls":  true,
		"previous_response_id": nullableString(state.PreviousResponseID),
		"conversation":         conversation,
		"background":           true,
		"store":                state.Store,
		"usage":                nil,
		"metadata":             map[string]string{},
	})
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
