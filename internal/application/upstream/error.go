package upstream

import (
	"encoding/json"
	"fmt"
)

func ErrorMessage(body []byte, status int) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if envelope.Error.Message != "" {
			return envelope.Error.Message
		}
		if envelope.Message != "" {
			return envelope.Message
		}
	}
	return fmt.Sprintf("upstream returned HTTP %d", status)
}
