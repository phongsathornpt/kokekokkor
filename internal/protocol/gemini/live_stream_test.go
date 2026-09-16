package gemini

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestLiveStreamDecoderTextTurn(t *testing.T) {
	decoder := NewLiveStreamDecoder("gemini-live")
	events, err := decoder.Decode(LiveServerMessage{
		ServerContent: &LiveServerContent{Text: []string{"hello", " world"}},
		Usage:         &llm.Usage{InputTokens: 5, OutputTokens: 2},
	})
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := []llm.StreamEventType{
		llm.StreamEventResponseStart,
		llm.StreamEventContentStart,
		llm.StreamEventTextDelta,
		llm.StreamEventTextDelta,
		llm.StreamEventUsage,
	}
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d: %#v", len(events), len(want), events)
	}
	for i, eventType := range want {
		if events[i].Type != eventType {
			t.Fatalf("event[%d].Type = %q, want %q", i, events[i].Type, eventType)
		}
	}
	if events[2].TextDelta != "hello" || events[3].TextDelta != " world" {
		t.Fatalf("text deltas = %q, %q", events[2].TextDelta, events[3].TextDelta)
	}
}

func TestLiveStreamDecoderCompletesTurn(t *testing.T) {
	decoder := NewLiveStreamDecoder("gemini-live")
	if _, err := decoder.Decode(LiveServerMessage{ServerContent: &LiveServerContent{Text: []string{"hello"}}}); err != nil {
		t.Fatalf("first Decode() error = %v", err)
	}
	events, err := decoder.Decode(LiveServerMessage{ServerContent: &LiveServerContent{TurnComplete: true}})
	if err != nil {
		t.Fatalf("completion Decode() error = %v", err)
	}
	if len(events) != 2 || events[0].Type != llm.StreamEventContentStop || events[1].Type != llm.StreamEventResponseStop {
		t.Fatalf("events = %#v", events)
	}
	if events[1].StopReason != llm.StopReasonEndTurn {
		t.Fatalf("StopReason = %q", events[1].StopReason)
	}
}

func TestLiveStreamDecoderInterruptedTurnUsesUnknownStop(t *testing.T) {
	decoder := NewLiveStreamDecoder("gemini-live")
	events, err := decoder.Decode(LiveServerMessage{ServerContent: &LiveServerContent{Interrupted: true}})
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(events) != 2 || events[0].Type != llm.StreamEventResponseStart || events[1].Type != llm.StreamEventResponseStop {
		t.Fatalf("events = %#v", events)
	}
	if events[1].StopReason != llm.StopReasonUnknown {
		t.Fatalf("StopReason = %q", events[1].StopReason)
	}
}

func TestLiveStreamDecoderIgnoresControlPlaneMessages(t *testing.T) {
	decoder := NewLiveStreamDecoder("gemini-live")
	for _, message := range []LiveServerMessage{
		{SetupComplete: true},
		{GoAway: json.RawMessage(`{"timeLeft":"10s"}`)},
		{SessionResumptionUpdate: json.RawMessage(`{"resumable":true}`)},
	} {
		events, err := decoder.Decode(message)
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("events = %#v", events)
		}
	}
}

func TestLiveStreamDecoderRejectsNonPortableSignals(t *testing.T) {
	decoder := NewLiveStreamDecoder("gemini-live")
	for _, message := range []LiveServerMessage{
		{ToolCall: json.RawMessage(`{"functionCalls":[]}`)},
		{Metadata: map[string]json.RawMessage{"future": json.RawMessage(`true`)}},
		{ServerContent: &LiveServerContent{Metadata: map[string]json.RawMessage{"groundingMetadata": json.RawMessage(`{}`)}}},
	} {
		if _, err := decoder.Decode(message); err == nil {
			t.Fatalf("message %#v accepted", message)
		}
	}
}
