package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestRealtimeServerEncoderTextSequence(t *testing.T) {
	encoder := NewRealtimeServerEncoder()
	var payloads [][]byte
	for _, event := range []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, Model: "gemini-live"},
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: "hello"},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: " world"},
		{Type: llm.StreamEventUsage, Usage: &llm.Usage{InputTokens: 7, OutputTokens: 2, CacheReadTokens: 3}},
		{Type: llm.StreamEventContentStop, Index: 0},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonEndTurn},
	} {
		encoded, err := encoder.Encode(event)
		if err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
		payloads = append(payloads, encoded...)
	}

	var types []string
	for _, payload := range payloads {
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v", payload, err)
		}
		types = append(types, event.Type)
	}
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.done",
	}
	if len(types) != len(want) {
		t.Fatalf("event types = %#v", types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("types[%d] = %q, want %q", i, types[i], want[i])
		}
	}

	var done struct {
		Response struct {
			Status string `json:"status"`
			Output []struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
			Usage struct {
				InputTokens  int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
				TotalTokens  int64 `json:"total_tokens"`
			} `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal(payloads[len(payloads)-1], &done); err != nil {
		t.Fatalf("decode response.done: %v", err)
	}
	if done.Response.Status != "completed" || len(done.Response.Output) != 1 || len(done.Response.Output[0].Content) != 1 {
		t.Fatalf("response.done = %#v", done.Response)
	}
	if done.Response.Output[0].Content[0].Text != "hello world" {
		t.Fatalf("final text = %q", done.Response.Output[0].Content[0].Text)
	}
	if done.Response.Usage.InputTokens != 7 || done.Response.Usage.OutputTokens != 2 || done.Response.Usage.TotalTokens != 9 {
		t.Fatalf("usage = %#v", done.Response.Usage)
	}
}

func TestRealtimeServerEncoderRejectsInvalidOrder(t *testing.T) {
	encoder := NewRealtimeServerEncoder()
	if _, err := encoder.Encode(llm.StreamEvent{Type: llm.StreamEventTextDelta, TextDelta: "nope"}); err == nil {
		t.Fatal("text delta before response accepted")
	}
	if _, err := encoder.Encode(llm.StreamEvent{Type: llm.StreamEventResponseStart}); err != nil {
		t.Fatalf("response start error = %v", err)
	}
	if _, err := encoder.Encode(llm.StreamEvent{Type: llm.StreamEventContentStart, Block: llm.ToolCallBlock{}}); err == nil {
		t.Fatal("tool content accepted by text bridge")
	}
}

func TestRealtimeServerEncoderRejectsUnsupportedStopReason(t *testing.T) {
	encoder := NewRealtimeServerEncoder()
	for _, event := range []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart},
		{Type: llm.StreamEventContentStart, Block: llm.TextBlock{}},
		{Type: llm.StreamEventContentStop},
	} {
		if _, err := encoder.Encode(event); err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
	}
	if _, err := encoder.Encode(llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonUnknown}); err == nil {
		t.Fatal("unknown stop reason accepted")
	}
}
