package anthropic

import (
	"encoding/json"
	"testing"
)

func TestMessagesReasoningControlsRoundTrip(t *testing.T) {
	input := []byte(`{
		"model":"claude-model",
		"max_tokens":4096,
		"thinking":{"type":"adaptive","display":"summarized"},
		"output_config":{"effort":"medium"},
		"messages":[{"role":"user","content":"hello"}]
	}`)
	request, err := DecodeMessagesRequest(input)
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}
	if request.Reasoning == nil {
		t.Fatal("Reasoning = nil")
	}
	if request.Reasoning.Mode != "adaptive" || request.Reasoning.Effort != "medium" || request.Reasoning.Summary != "auto" {
		t.Fatalf("Reasoning = %#v", request.Reasoning)
	}

	encoded, err := EncodeMessagesRequest(request)
	if err != nil {
		t.Fatalf("EncodeMessagesRequest() error = %v", err)
	}
	var wire messagesRequest
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	var thinking map[string]any
	if err := json.Unmarshal(wire.Thinking, &thinking); err != nil {
		t.Fatalf("decode thinking: %v", err)
	}
	if thinking["type"] != "adaptive" || thinking["display"] != "summarized" {
		t.Fatalf("thinking = %#v", thinking)
	}
	var outputConfig map[string]any
	if err := json.Unmarshal(wire.OutputConfig, &outputConfig); err != nil {
		t.Fatalf("decode output_config: %v", err)
	}
	if outputConfig["effort"] != "medium" {
		t.Fatalf("output_config = %#v", outputConfig)
	}
}
