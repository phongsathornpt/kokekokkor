package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeChatRequestMapsPortableSemantics(t *testing.T) {
	input := []byte(`{
		"model":"client-model",
		"messages":[
			{"role":"developer","content":"be exact"},
			{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]},
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"done"}
		],
		"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],
		"reasoning_effort":"high",
		"x-extra":true
	}`)

	request, err := DecodeChatRequest(input)
	if err != nil {
		t.Fatalf("DecodeChatRequest() error = %v", err)
	}
	if request.Model != "client-model" || len(request.Messages) != 4 {
		t.Fatalf("request = %#v", request)
	}
	if request.Messages[0].Role != llm.RoleDeveloper {
		t.Fatalf("role = %q", request.Messages[0].Role)
	}
	if _, ok := request.Messages[1].Content[0].(llm.ImageBlock); !ok {
		t.Fatalf("image = %#v", request.Messages[1].Content)
	}
	if _, ok := request.Messages[2].Content[0].(llm.ToolCallBlock); !ok {
		t.Fatalf("tool call = %#v", request.Messages[2].Content)
	}
	if _, ok := request.Messages[3].Content[0].(llm.ToolResultBlock); !ok {
		t.Fatalf("tool result = %#v", request.Messages[3].Content)
	}
	if request.Reasoning == nil || request.Reasoning.Effort != "high" {
		t.Fatalf("reasoning = %#v", request.Reasoning)
	}
	if _, ok := request.Metadata["x-extra"]; !ok {
		t.Fatal("unknown metadata was not preserved")
	}
}

func TestEncodeChatRequestPreservesMetadataAndToolResult(t *testing.T) {
	request := llm.Request{
		Model: "upstream-model",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{llm.ToolResultBlock{
				ToolCallID: "call_1",
				Content:    []llm.ContentBlock{llm.TextBlock{Text: "done"}},
			}},
		}},
		Metadata: map[string]json.RawMessage{"x-extra": json.RawMessage(`true`)},
	}

	encoded, err := EncodeChatRequest(request)
	if err != nil {
		t.Fatalf("EncodeChatRequest() error = %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := wire["x-extra"]; !ok {
		t.Fatalf("metadata missing: %s", encoded)
	}
	var messages []chatMessage
	if err := json.Unmarshal(wire["messages"], &messages); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Role != "tool" || messages[0].ToolCallID != "call_1" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestChatCodecRoundTripKeepsCoreSemantics(t *testing.T) {
	input := []byte(`{"model":"m","messages":[{"role":"user","content":"hello"}],"stop":["END","STOP"]}`)
	request, err := DecodeChatRequest(input)
	if err != nil {
		t.Fatalf("DecodeChatRequest() error = %v", err)
	}
	encoded, err := EncodeChatRequest(request)
	if err != nil {
		t.Fatalf("EncodeChatRequest() error = %v", err)
	}
	decoded, err := DecodeChatRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeChatRequest(round trip) error = %v", err)
	}
	if decoded.Model != "m" || len(decoded.Stop) != 2 || decoded.Stop[1] != "STOP" {
		t.Fatalf("decoded = %#v", decoded)
	}
}
