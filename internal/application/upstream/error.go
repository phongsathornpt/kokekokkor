package upstream

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxPublicErrorMessageBytes = 4 << 10

func ErrorMessage(body []byte, status int) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if message := publicErrorMessage(envelope.Error.Message); message != "" {
			return message
		}
		if message := publicErrorMessage(envelope.Message); message != "" {
			return message
		}
	}
	return fmt.Sprintf("upstream returned HTTP %d", status)
}

func publicErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" || len(message) <= maxPublicErrorMessageBytes {
		return message
	}
	cut := maxPublicErrorMessageBytes
	for cut > 0 && !utf8.RuneStart(message[cut]) {
		cut--
	}
	return strings.TrimSpace(message[:cut]) + "…"
}
