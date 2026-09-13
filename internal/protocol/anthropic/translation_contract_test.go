package anthropic_test

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

func TestOpenAIToAnthropicContractPreservesPortableSemantics(t *testing.T) {
	openAIInput := []byte(`{
		"model":"portable-model",
		"max_completion_tokens":256,
		"messages":[
			{"role":"developer","content":"be precise"},
			{"role":"user","content":[{"type":"text","text":"inspect"},{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]},
			{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"done"}
		],
		"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],
		"tool_choice":"auto"
	}`)

	canonical, err := openaiProtocol.DecodeChatRequest(openAIInput)
	if err != nil {
		t.Fatalf("DecodeChatRequest() error = %v", err)
	}
	anthropicWire, err := anthropicProtocol.EncodeMessagesRequest(canonical)
	if err != nil {
		t.Fatalf("EncodeMessagesRequest() error = %v", err)
	}
	decoded, err := anthropicProtocol.DecodeMessagesRequest(anthropicWire)
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}

	if decoded.Model != "portable-model" || len(decoded.Messages) != 4 {
		t.Fatalf("decoded = %#v", decoded)
	}
	if decoded.Messages[0].Role != llm.RoleSystem {
		t.Fatalf("instruction role = %q, want system after Anthropic lift", decoded.Messages[0].Role)
	}
	image, ok := decoded.Messages[1].Content[1].(llm.ImageBlock)
	if !ok || image.Source.Type != llm.MediaSourceBase64 || image.Source.MediaType != "image/png" {
		t.Fatalf("image = %#v", decoded.Messages[1].Content[1])
	}
	call, ok := decoded.Messages[2].Content[0].(llm.ToolCallBlock)
	if !ok || call.Name != "lookup" || call.ID != "call_1" {
		t.Fatalf("tool call = %#v", decoded.Messages[2].Content)
	}
	result, ok := decoded.Messages[3].Content[0].(llm.ToolResultBlock)
	if !ok || result.ToolCallID != "call_1" {
		t.Fatalf("tool result = %#v", decoded.Messages[3].Content)
	}
}

func TestAnthropicToOpenAIContractPreservesPortableSemantics(t *testing.T) {
	anthropicInput := []byte(`{
		"model":"portable-model",
		"max_tokens":256,
		"system":"be precise",
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"x"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"done"}]}
		],
		"tools":[{"name":"lookup","input_schema":{"type":"object"}}],
		"tool_choice":{"type":"auto"}
	}`)

	canonical, err := anthropicProtocol.DecodeMessagesRequest(anthropicInput)
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}
	openAIWire, err := openaiProtocol.EncodeChatRequest(canonical)
	if err != nil {
		t.Fatalf("EncodeChatRequest() error = %v", err)
	}
	decoded, err := openaiProtocol.DecodeChatRequest(openAIWire)
	if err != nil {
		t.Fatalf("DecodeChatRequest() error = %v", err)
	}

	if decoded.Model != "portable-model" || len(decoded.Messages) != 4 {
		t.Fatalf("decoded = %#v", decoded)
	}
	if decoded.Messages[0].Role != llm.RoleSystem {
		t.Fatalf("system role = %q", decoded.Messages[0].Role)
	}
	call, ok := decoded.Messages[2].Content[0].(llm.ToolCallBlock)
	if !ok || call.Name != "lookup" || call.ID != "toolu_1" {
		t.Fatalf("tool call = %#v", decoded.Messages[2].Content)
	}
	result, ok := decoded.Messages[3].Content[0].(llm.ToolResultBlock)
	if !ok || result.ToolCallID != "toolu_1" {
		t.Fatalf("tool result = %#v", decoded.Messages[3].Content)
	}
}
