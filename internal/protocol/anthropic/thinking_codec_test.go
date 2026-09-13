package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeMessagesRequestPreservesAdaptiveThinking(t *testing.T) {
	request, err := DecodeMessagesRequest([]byte(`{
		"model":"claude-model",
		"max_tokens":128,
		"thinking":{"type":"adaptive"},
		"messages":[{"role":"user","content":"hello"}]
	}`))
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}
	if request.Reasoning == nil || !request.Reasoning.Enabled || request.Reasoning.Mode != "adaptive" {
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
	var thinking map[string]json.RawMessage
	if err := json.Unmarshal(wire.Thinking, &thinking); err != nil {
		t.Fatalf("decode thinking: %v", err)
	}
	if string(thinking["type"]) != `"adaptive"` {
		t.Fatalf("thinking.type = %s", thinking["type"])
	}
	if _, exists := thinking["budget_tokens"]; exists {
		t.Fatalf("adaptive thinking unexpectedly encoded budget_tokens: %s", wire.Thinking)
	}
}

func TestMessagesCodecPreservesRedactedThinkingBlock(t *testing.T) {
	input := []byte(`{
		"model":"claude-model",
		"max_tokens":128,
		"messages":[
			{"role":"assistant","content":[{"type":"redacted_thinking","data":"opaque-payload"}]}
		]
	}`)

	request, err := DecodeMessagesRequest(input)
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}
	block, ok := request.Messages[0].Content[0].(llm.ReasoningBlock)
	if !ok || block.RedactedData != "opaque-payload" {
		t.Fatalf("reasoning block = %#v", request.Messages[0].Content[0])
	}

	encoded, err := EncodeMessagesRequest(request)
	if err != nil {
		t.Fatalf("EncodeMessagesRequest() error = %v", err)
	}
	var wire messagesRequest
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	var blocks []anthropicContentBlock
	if err := json.Unmarshal(wire.Messages[0].Content, &blocks); err != nil {
		t.Fatalf("decode content: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Type != "redacted_thinking" || blocks[0].Data != "opaque-payload" {
		t.Fatalf("blocks = %#v", blocks)
	}
}
