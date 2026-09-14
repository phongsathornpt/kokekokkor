package openai

import (
	"bytes"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestResponsesStreamEncoderEmitsFailedResponse(t *testing.T) {
	var out bytes.Buffer
	encoder := NewResponsesStreamEncoder(&out, false)
	if err := encoder.Encode(llm.StreamEvent{Type: llm.StreamEventResponseStart, ResponseID: "msg_1", Model: "claude"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := encoder.Encode(llm.StreamEvent{
		Type: llm.StreamEventError,
		Error: &llm.StreamError{Code: "overloaded_error", Message: "upstream overloaded", Retryable: true},
	}); err != nil {
		t.Fatalf("error: %v", err)
	}

	body := out.String()
	for _, want := range []string{`event: response.failed`, `"status":"failed"`, `"code":"overloaded_error"`, `"message":"upstream overloaded"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream missing %q:\n%s", want, body)
		}
	}
}
