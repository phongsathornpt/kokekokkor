package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestLiveClientEncoderFunctionToolsAndResult(t *testing.T) {
	encoder := NewLiveClientEncoder("gemini-live")
	setup, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventSessionUpdate,
		SessionConfig: &llm.RealtimeSessionConfig{
			OutputModalities: []string{"text"},
			Tools: []llm.Tool{{
				Name:        "weather",
				Description: "Get weather",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
			}},
			ToolChoice: &llm.ToolChoice{Mode: llm.ToolChoiceNamed, Name: "weather"},
		},
	})
	if err != nil {
		t.Fatalf("setup Encode() error = %v", err)
	}
	for _, want := range []string{`"functionDeclarations"`, `"name":"weather"`, `"mode":"ANY"`, `"allowedFunctionNames":["weather"]`} {
		if !strings.Contains(string(setup), want) {
			t.Fatalf("setup missing %q: %s", want, setup)
		}
	}

	result, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventItemCreate,
		ToolResult: &llm.ToolResultBlock{
			ToolCallID: "call_1",
			Name:       "weather",
			Content:    []llm.ContentBlock{llm.TextBlock{Text: `{"temp":31}`}},
		},
	})
	if err != nil {
		t.Fatalf("tool result Encode() error = %v", err)
	}
	for _, want := range []string{`"toolResponse"`, `"id":"call_1"`, `"name":"weather"`, `"temp":31`} {
		if !strings.Contains(string(result), want) {
			t.Fatalf("tool response missing %q: %s", want, result)
		}
	}
}

func TestLiveStreamDecoderFunctionCall(t *testing.T) {
	decoder := NewLiveStreamDecoder("gemini-live")
	events, err := decoder.Decode(LiveServerMessage{ToolCall: json.RawMessage(`{"functionCalls":[{"id":"call_1","name":"weather","args":{"city":"Bangkok"}}]}`)})
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("len(events) = %d, events = %#v", len(events), events)
	}
	if events[0].Type != llm.StreamEventResponseStart || events[1].Type != llm.StreamEventToolCallStart || events[2].Type != llm.StreamEventToolCallDelta || events[3].Type != llm.StreamEventContentStop || events[4].Type != llm.StreamEventResponseStop {
		t.Fatalf("events = %#v", events)
	}
	call, ok := events[1].Block.(llm.ToolCallBlock)
	if !ok || call.ID != "call_1" || call.Name != "weather" {
		t.Fatalf("tool call = %#v", events[1].Block)
	}
	if events[2].ToolCallDelta == nil || events[2].ToolCallDelta.ArgumentsDelta != `{"city":"Bangkok"}` {
		t.Fatalf("tool delta = %#v", events[2].ToolCallDelta)
	}
	if events[4].StopReason != llm.StopReasonToolUse {
		t.Fatalf("stop reason = %q", events[4].StopReason)
	}
}
