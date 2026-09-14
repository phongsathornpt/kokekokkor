package anthropic

import (
	"bytes"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeMessagesStreamEmitsPortableError(t *testing.T) {
	input := strings.NewReader("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude\",\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\nevent: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"busy\"}}\n\n")
	var events []llm.StreamEvent
	if err := DecodeMessagesStream(input, func(event llm.StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("DecodeMessagesStream() error = %v", err)
	}
	last := events[len(events)-1]
	if last.Type != llm.StreamEventError || last.Error == nil {
		t.Fatalf("last event = %#v", last)
	}
	if last.Error.Code != "overloaded_error" || last.Error.Message != "busy" || !last.Error.Retryable {
		t.Fatalf("stream error = %#v", last.Error)
	}
}

func TestMessagesStreamEncoderEmitsErrorEvent(t *testing.T) {
	var out bytes.Buffer
	encoder := NewMessagesStreamEncoder(&out)
	if err := encoder.Encode(llm.StreamEvent{Type: llm.StreamEventError, Error: &llm.StreamError{Code: "overloaded_error", Message: "busy"}}); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	body := out.String()
	for _, want := range []string{"event: error", `"type":"overloaded_error"`, `"message":"busy"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream missing %q:\n%s", want, body)
		}
	}
}
