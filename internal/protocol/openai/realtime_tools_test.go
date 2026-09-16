package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeRealtimeSessionFunctionTools(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{
		"type":"session.update",
		"session":{
			"model":"portable",
			"output_modalities":["text"],
			"tools":[{"type":"function","name":"weather","description":"Get weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}],
			"tool_choice":{"type":"function","name":"weather"}
		}
	}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.SessionConfig == nil || len(event.SessionConfig.Tools) != 1 {
		t.Fatalf("SessionConfig = %#v", event.SessionConfig)
	}
	tool := event.SessionConfig.Tools[0]
	if tool.Name != "weather" || tool.Description != "Get weather" || !json.Valid(tool.InputSchema) {
		t.Fatalf("tool = %#v", tool)
	}
	if event.SessionConfig.ToolChoice == nil || event.SessionConfig.ToolChoice.Mode != llm.ToolChoiceNamed || event.SessionConfig.ToolChoice.Name != "weather" {
		t.Fatalf("ToolChoice = %#v", event.SessionConfig.ToolChoice)
	}
}

func TestDecodeRealtimeFunctionCallOutput(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{"type":"conversation.item.create","item":{"type":"function_call_output","call_id":"call_1","output":"{\"temp\":31}"}}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.ToolResult == nil || event.ToolResult.ToolCallID != "call_1" || len(event.ToolResult.Content) != 1 {
		t.Fatalf("ToolResult = %#v", event.ToolResult)
	}
	text, ok := event.ToolResult.Content[0].(llm.TextBlock)
	if !ok || text.Text != `{"temp":31}` {
		t.Fatalf("tool result content = %#v", event.ToolResult.Content)
	}
}

func TestRealtimeServerEncoderFunctionCall(t *testing.T) {
	encoder := NewRealtimeServerEncoder()
	var payloads [][]byte
	appendEvents := func(event llm.StreamEvent) {
		events, err := encoder.Encode(event)
		if err != nil {
			t.Fatalf("Encode(%s) error = %v", event.Type, err)
		}
		payloads = append(payloads, events...)
	}
	appendEvents(llm.StreamEvent{Type: llm.StreamEventResponseStart, Model: "gemini-live"})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventToolCallStart, Index: 0, Block: llm.ToolCallBlock{ID: "call_1", Name: "weather", Arguments: json.RawMessage(`{}`)}})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventToolCallDelta, Index: 0, ToolCallDelta: &llm.ToolCallDelta{ID: "call_1", Name: "weather", ArgumentsDelta: `{"city":"Bangkok"}`}})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventContentStop, Index: 0})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonToolUse})

	joined := ""
	for _, payload := range payloads {
		joined += string(payload) + "\n"
	}
	for _, want := range []string{"response.function_call_arguments.delta", "response.function_call_arguments.done", `"type":"function_call"`, `"call_id":"call_1"`, `"name":"weather"`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("payloads missing %q:\n%s", want, joined)
		}
	}
}
