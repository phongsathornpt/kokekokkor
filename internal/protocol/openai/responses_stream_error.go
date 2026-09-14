package openai

import (
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func (e *ResponsesStreamEncoder) failResponse(event llm.StreamEvent) error {
	if !e.started || e.stopped {
		return fmt.Errorf("stream error outside active Responses stream")
	}
	if event.Error == nil {
		return fmt.Errorf("stream error is missing error payload")
	}

	e.stopped = true
	code := event.Error.Code
	if code == "" {
		code = "server_error"
	}
	snapshot := e.snapshot("failed", nil, false)
	snapshot.Error = map[string]any{
		"code":    code,
		"message": event.Error.Message,
	}
	return e.write("response.failed", map[string]any{
		"response": snapshot,
	})
}
