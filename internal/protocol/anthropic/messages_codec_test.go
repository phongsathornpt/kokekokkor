package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeMessagesRequestMapsPortableSemantics(t *testing.T) {
	data := []byte(`{
		"model":"claude-model",
		"max_tokens":512,
		"system":[{"type":"text","text":"be exact"}],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"inspect"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}]},
			{"role":"assistant","content":[{"type":"thinking","thinking":"hidden","signature":"sig"},{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"x"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"done","is_error":false}]}
		],
		"tools":[{"name":"lookup","description":"Lookup","input_schema":{"type":"object"}}],
		"tool_choice":{"type":"tool","name":"lookup","disable_parallel_tool_use":true},
		"thinking":{"type":"enabled","budget_tokens":2048,"vendor_mode":"x"},
		"stop_sequences":["END"],
		"x-provider-option":{"enabled":true}
	}`)

	request, err := DecodeMessagesRequest(data)
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}
	if request.Model != "claude-model" || len(request.Messages) != 4 {
		t.Fatalf("request = %#v", request)
	}
	if request.Messages[0].Role != llm.RoleSystem {
		t.Fatalf("system role = %q", request.Messages[0].Role)
	}
	image, ok := request.Messages[1].Content[1].(llm.ImageBlock)
	if !ok || image.Source.Type != llm.MediaSourceBase64 {
		t.Fatalf("image block = %#v", request.Messages[1].Content[1])
	}
	reasoning, ok := request.Messages[2].Content[0].(llm.ReasoningBlock)
	if !ok || reasoning.Signature != "sig" {
		t.Fatalf("reasoning block = %#v", request.Messages[2].Content[0])
	}
	call, ok := request.Messages[2].Content[1].(llm.ToolCallBlock)
	if !ok || call.Name != "lookup" {
		t.Fatalf("tool call = %#v", request.Messages[2].Content[1])
	}
	result, ok := request.Messages[3].Content[0].(llm.ToolResultBlock)
	if !ok || result.ToolCallID != "toolu_1" {
		t.Fatalf("tool result = %#v", request.Messages[3].Content[0])
	}
	if request.ToolChoice == nil || request.ToolChoice.Mode != llm.ToolChoiceNamed || !request.ToolChoice.DisableParallel {
		t.Fatalf("tool choice = %#v", request.ToolChoice)
	}
	if request.Reasoning == nil || !request.Reasoning.Enabled || request.Reasoning.BudgetTokens != 2048 {
		t.Fatalf("reasoning config = %#v", request.Reasoning)
	}
	if _, ok := request.Reasoning.Metadata["vendor_mode"]; !ok {
		t.Fatalf("thinking metadata missing: %#v", request.Reasoning.Metadata)
	}
	if _, ok := request.Metadata["x-provider-option"]; !ok {
		t.Fatalf("provider metadata missing: %#v", request.Metadata)
	}
}

func TestEncodeMessagesRequestLiftsInstructionsAndToolResults(t *testing.T) {
	maxTokens := 256
	request := llm.Request{
		Model:           "claude-upstream",
		MaxOutputTokens: &maxTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: []llm.ContentBlock{llm.TextBlock{Text: "system"}}},
			{Role: llm.RoleDeveloper, Content: []llm.ContentBlock{llm.TextBlock{Text: "developer"}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}}},
			{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
				llm.ToolCallBlock{ID: "toolu_1", Name: "lookup", Arguments: []byte(`{"q":"x"}`)},
			}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{
				llm.ToolResultBlock{ToolCallID: "toolu_1", Content: []llm.ContentBlock{llm.TextBlock{Text: "done"}}},
			}},
		},
		Tools: []llm.Tool{{Name: "lookup", InputSchema: []byte(`{"type":"object"}`)}},
		Metadata: map[string]json.RawMessage{
			"x-provider-option": []byte(`{"enabled":true}`),
		},
	}

	encoded, err := EncodeMessagesRequest(request)
	if err != nil {
		t.Fatalf("EncodeMessagesRequest() error = %v", err)
	}

	var wire messagesRequest
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(wire.Messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3: %s", len(wire.Messages), encoded)
	}
	var system []anthropicContentBlock
	if err := json.Unmarshal(wire.System, &system); err != nil {
		t.Fatalf("decode system: %v", err)
	}
	if len(system) != 2 || system[0].Text != "system" || system[1].Text != "developer" {
		t.Fatalf("system = %#v", system)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("decode encoded request: %v", err)
	}
	if _, ok := raw["x-provider-option"]; !ok {
		t.Fatalf("metadata missing: %s", encoded)
	}
}

func TestMessagesCodecRoundTripKeepsCoreSemantics(t *testing.T) {
	input := []byte(`{
		"model":"m",
		"max_tokens":64,
		"messages":[{"role":"user","content":"hello"}],
		"stop_sequences":["END"]
	}`)

	request, err := DecodeMessagesRequest(input)
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}
	encoded, err := EncodeMessagesRequest(request)
	if err != nil {
		t.Fatalf("EncodeMessagesRequest() error = %v", err)
	}
	decoded, err := DecodeMessagesRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeMessagesRequest(round trip) error = %v", err)
	}
	if decoded.Model != "m" || len(decoded.Stop) != 1 || decoded.Stop[0] != "END" {
		t.Fatalf("round trip request = %#v", decoded)
	}
}

func TestEncodeMessagesRequestRequiresMaxTokens(t *testing.T) {
	_, err := EncodeMessagesRequest(llm.Request{Model: "m"})
	if err == nil {
		t.Fatal("EncodeMessagesRequest() error = nil, want max token error")
	}
}
