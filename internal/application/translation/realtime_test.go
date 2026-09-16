package translation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestOpenAIRealtimeToGeminiTextEventPortableSubset(t *testing.T) {
	tests := []llm.RealtimeEvent{
		{Type: llm.RealtimeEventSessionStart, WireType: "session.start", Session: json.RawMessage(`{"model":"gpt-realtime"}`)},
		{Type: llm.RealtimeEventSessionUpdate, WireType: "session.update", Session: json.RawMessage(`{"instructions":"be concise"}`)},
		{Type: llm.RealtimeEventItemCreate, WireType: "conversation.item.create", Item: json.RawMessage(`{"type":"message"}`)},
		{Type: llm.RealtimeEventResponseCreate, WireType: "response.create"},
	}

	for _, event := range tests {
		if err := OpenAIRealtimeToGeminiTextEvent(event); err != nil {
			t.Fatalf("event %q rejected: %v", event.Type, err)
		}
	}
}

func TestOpenAIRealtimeToGeminiTextEventRejectsNonPortableControls(t *testing.T) {
	tests := []llm.RealtimeEvent{
		{Type: llm.RealtimeEventInputAudioAppend, WireType: "input_audio_buffer.append", Audio: "YWJj"},
		{Type: llm.RealtimeEventInputAudioCommit, WireType: "input_audio_buffer.commit"},
		{Type: llm.RealtimeEventInputAudioClear, WireType: "input_audio_buffer.clear"},
		{Type: llm.RealtimeEventResponseCancel, WireType: "response.cancel"},
		{Type: llm.RealtimeEventUnknown, WireType: "vendor.future.event"},
	}

	for _, event := range tests {
		err := OpenAIRealtimeToGeminiTextEvent(event)
		if err == nil {
			t.Fatalf("event %q accepted, want compatibility error", event.Type)
		}
		var compatibilityErr CompatibilityError
		if !errors.As(err, &compatibilityErr) {
			t.Fatalf("event %q error = %T, want CompatibilityError", event.Type, err)
		}
	}
}

func TestOpenAIRealtimeToGeminiTextEventRejectsProviderMetadata(t *testing.T) {
	err := OpenAIRealtimeToGeminiTextEvent(llm.RealtimeEvent{
		Type:     llm.RealtimeEventResponseCreate,
		WireType: "response.create",
		Metadata: map[string]json.RawMessage{"provider_extension": json.RawMessage(`true`)},
	})
	if err == nil {
		t.Fatal("provider metadata accepted, want compatibility error")
	}
}

func TestOpenAIRealtimeToGeminiTextEventRequiresPayloads(t *testing.T) {
	for _, event := range []llm.RealtimeEvent{
		{Type: llm.RealtimeEventSessionUpdate, WireType: "session.update"},
		{Type: llm.RealtimeEventItemCreate, WireType: "conversation.item.create"},
	} {
		if err := OpenAIRealtimeToGeminiTextEvent(event); err == nil {
			t.Fatalf("event %q missing payload accepted", event.Type)
		}
	}
}
