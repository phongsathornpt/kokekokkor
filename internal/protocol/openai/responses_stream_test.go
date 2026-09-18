package openai

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/sse"
)

func TestResponsesStreamEncoderTextLifecycle(t *testing.T) {
	var buffer bytes.Buffer
	encoder := NewResponsesStreamEncoder(&buffer, false)
	usage := llm.Usage{InputTokens: 4, OutputTokens: 2, CacheReadTokens: 1, CacheWriteTokens: 3}
	events := []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, ResponseID: "msg_1", Model: "claude"},
		{Type: llm.StreamEventUsage, Usage: &llm.Usage{InputTokens: 4}},
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: "hel"},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: "lo"},
		{Type: llm.StreamEventContentStop, Index: 0},
		{Type: llm.StreamEventUsage, Usage: &usage},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonEndTurn},
	}
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
	}

	var names []string
	var terminal map[string]json.RawMessage
	if err := sse.Decode(strings.NewReader(buffer.String()), func(event sse.Event) error {
		names = append(names, event.Name)
		if event.Name == "response.completed" {
			return json.Unmarshal(event.Data, &terminal)
		}
		return nil
	}); err != nil {
		t.Fatalf("decode generated SSE: %v", err)
	}

	want := []string{
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("event names = %#v, want %#v", names, want)
	}
	if strings.Contains(buffer.String(), "[DONE]") {
		t.Fatalf("Responses stream contains Chat Completions terminator: %q", buffer.String())
	}

	var response struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		OutputText string `json:"output_text"`
		Usage      struct {
			InputTokens        int64 `json:"input_tokens"`
			OutputTokens       int64 `json:"output_tokens"`
			InputTokensDetails struct {
				CachedTokens     int64 `json:"cached_tokens"`
				CacheWriteTokens int64 `json:"cache_write_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(terminal["response"], &response); err != nil {
		t.Fatalf("decode terminal response: %v", err)
	}
	if response.ID != "resp_msg_1" || response.Status != "completed" || response.OutputText != "hello" {
		t.Fatalf("terminal response = %#v", response)
	}
	if response.Usage.InputTokens != 4 || response.Usage.OutputTokens != 2 || response.Usage.InputTokensDetails.CachedTokens != 1 || response.Usage.InputTokensDetails.CacheWriteTokens != 3 {
		t.Fatalf("terminal usage = %#v", response.Usage)
	}
}

func TestResponsesStreamEncoderFunctionCallLifecycle(t *testing.T) {
	var buffer bytes.Buffer
	encoder := NewResponsesStreamEncoder(&buffer, false)
	events := []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, ResponseID: "msg_2", Model: "claude"},
		{Type: llm.StreamEventToolCallStart, Index: 0, Block: llm.ToolCallBlock{ID: "call_1", Name: "weather"}},
		{Type: llm.StreamEventToolCallDelta, Index: 0, ToolCallDelta: &llm.ToolCallDelta{ArgumentsDelta: `{"city":`}},
		{Type: llm.StreamEventToolCallDelta, Index: 0, ToolCallDelta: &llm.ToolCallDelta{ArgumentsDelta: `"BKK"}`}},
		{Type: llm.StreamEventContentStop, Index: 0},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonToolUse},
	}
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
	}

	var names []string
	var done struct {
		Arguments string `json:"arguments"`
		Name      string `json:"name"`
	}
	if err := sse.Decode(strings.NewReader(buffer.String()), func(event sse.Event) error {
		names = append(names, event.Name)
		if event.Name == "response.function_call_arguments.done" {
			return json.Unmarshal(event.Data, &done)
		}
		return nil
	}); err != nil {
		t.Fatalf("decode generated SSE: %v", err)
	}
	for _, name := range []string{
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	} {
		if !containsString(names, name) {
			t.Fatalf("stream missing %s: %#v", name, names)
		}
	}
	if done.Name != "weather" || done.Arguments != `{"city":"BKK"}` {
		t.Fatalf("function call done = %#v", done)
	}
}

func TestResponsesStreamEncoderRefusalLifecycle(t *testing.T) {
	var buffer bytes.Buffer
	encoder := NewResponsesStreamEncoder(&buffer, false)
	encoder.SetRefusalMode(true)
	for _, event := range []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, ResponseID: "msg_refusal", Model: "claude"},
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: "I can't help with that."},
		{Type: llm.StreamEventContentStop, Index: 0},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonContentBlock},
	} {
		if err := encoder.Encode(event); err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
	}
	text := buffer.String()
	for _, want := range []string{"response.refusal.delta", "response.refusal.done", "response.completed", `"type":"refusal"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("refusal stream missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "response.output_text.delta") || strings.Contains(text, `"output_text":"I can't help with that."`) {
		t.Fatalf("refusal leaked as output text: %s", text)
	}
}

func TestResponsesStreamEncoderObfuscatesDeltasByDefault(t *testing.T) {
	var buffer bytes.Buffer
	encoder := NewResponsesStreamEncoder(&buffer, true)
	for _, event := range []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, ResponseID: "msg_3", Model: "claude"},
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: "hi"},
	} {
		if err := encoder.Encode(event); err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
	}
	if !strings.Contains(buffer.String(), `"obfuscation":`) {
		t.Fatalf("delta has no obfuscation: %s", buffer.String())
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
