package gemini

import (
	"bytes"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/sse"
)

func TestDecodeGenerateContentStreamTextAndUsage(t *testing.T) {
	input := strings.Join([]string{
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"hel"}]}}],"modelVersion":"gemini-upstream","responseId":"resp_1"}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"lo"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2},"modelVersion":"gemini-upstream","responseId":"resp_1"}`,
		``,
	}, "\n")
	var events []llm.StreamEvent
	if err := DecodeGenerateContentStream(strings.NewReader(input), func(event llm.StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("DecodeGenerateContentStream() error = %v", err)
	}
	var text strings.Builder
	var stop llm.StopReason
	var usage *llm.Usage
	for _, event := range events {
		switch event.Type {
		case llm.StreamEventTextDelta:
			text.WriteString(event.TextDelta)
		case llm.StreamEventUsage:
			usage = event.Usage
		case llm.StreamEventResponseStop:
			stop = event.StopReason
		}
	}
	if text.String() != "hello" || stop != llm.StopReasonEndTurn {
		t.Fatalf("text=%q stop=%q events=%#v", text.String(), stop, events)
	}
	if usage == nil || usage.InputTokens != 4 || usage.OutputTokens != 2 {
		t.Fatalf("usage=%#v", usage)
	}
}

func TestDecodeGenerateContentStreamToolCallUsesToolStop(t *testing.T) {
	input := "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"name\":\"weather\",\"args\":{\"city\":\"Bangkok\"}}}]},\"finishReason\":\"STOP\"}],\"modelVersion\":\"gemini\",\"responseId\":\"resp_2\"}\n\n"
	var events []llm.StreamEvent
	if err := DecodeGenerateContentStream(strings.NewReader(input), func(event llm.StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("DecodeGenerateContentStream() error = %v", err)
	}
	if len(events) < 4 || events[len(events)-1].StopReason != llm.StopReasonToolUse {
		t.Fatalf("events=%#v", events)
	}
}

func TestGenerateContentStreamEncoderWritesNativeSSEWithoutDoneSentinel(t *testing.T) {
	var out bytes.Buffer
	encoder := NewGenerateContentStreamEncoder(&out)
	events := []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, ResponseID: "resp_1", Model: "gemini-upstream"},
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: "hello"},
		{Type: llm.StreamEventContentStop, Index: 0},
		{Type: llm.StreamEventUsage, Usage: &llm.Usage{InputTokens: 4, OutputTokens: 2}},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonEndTurn},
	}
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
	}
	if strings.Contains(out.String(), "[DONE]") {
		t.Fatalf("Gemini generateContent stream must not emit [DONE]: %q", out.String())
	}
	var chunks int
	var finish string
	if err := sse.Decode(strings.NewReader(out.String()), func(event sse.Event) error {
		chunks++
		if strings.Contains(string(event.Data), `"finishReason":"STOP"`) {
			finish = "STOP"
		}
		return nil
	}); err != nil {
		t.Fatalf("decode encoded SSE: %v", err)
	}
	if chunks != 2 || finish != "STOP" {
		t.Fatalf("chunks=%d finish=%q output=%q", chunks, finish, out.String())
	}
}
