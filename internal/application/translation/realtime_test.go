package translation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestOpenAIRealtimeToGeminiTextEventPortableSubset(t *testing.T) {
	tests := []llm.RealtimeEvent{
		{
			Type:     llm.RealtimeEventSessionStart,
			WireType: "session.start",
			SessionConfig: &llm.RealtimeSessionConfig{
				Model:            "gpt-realtime",
				OutputModalities: []string{"text"},
			},
		},
		{
			Type:     llm.RealtimeEventSessionUpdate,
			WireType: "session.update",
			SessionConfig: &llm.RealtimeSessionConfig{
				Instructions:         "be concise",
				OutputModalities:     []string{"audio"},
				InputAudioMediaType:  "audio/pcm;rate=24000",
				OutputAudioMediaType: "audio/pcm;rate=24000",
			},
		},
		{
			Type:     llm.RealtimeEventItemCreate,
			WireType: "conversation.item.create",
			Message: &llm.Message{
				Role:    llm.RoleUser,
				Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
			},
		},
		{Type: llm.RealtimeEventInputAudioAppend, WireType: "input_audio_buffer.append", Audio: "YWJj"},
		{Type: llm.RealtimeEventInputAudioCommit, WireType: "input_audio_buffer.commit"},
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
		{Type: llm.RealtimeEventInputAudioAppend, WireType: "input_audio_buffer.append", Audio: "%%%"},
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

	err = OpenAIRealtimeToGeminiTextEvent(llm.RealtimeEvent{
		Type:     llm.RealtimeEventSessionUpdate,
		WireType: "session.update",
		SessionConfig: &llm.RealtimeSessionConfig{
			OutputModalities: []string{"text"},
			Metadata:         map[string]json.RawMessage{"voice": json.RawMessage(`"alloy"`)},
		},
	})
	if err == nil {
		t.Fatal("provider session metadata accepted, want compatibility error")
	}
}

func TestOpenAIRealtimeToGeminiTextEventRequiresCanonicalPayloads(t *testing.T) {
	for _, event := range []llm.RealtimeEvent{
		{Type: llm.RealtimeEventSessionUpdate, WireType: "session.update", Session: json.RawMessage(`{"output_modalities":["text"]}`)},
		{Type: llm.RealtimeEventItemCreate, WireType: "conversation.item.create", Item: json.RawMessage(`{"type":"message"}`)},
	} {
		if err := OpenAIRealtimeToGeminiTextEvent(event); err == nil {
			t.Fatalf("event %q missing canonical payload accepted", event.Type)
		}
	}
}

func TestOpenAIRealtimeToGeminiTextEventRequiresSinglePortableOutput(t *testing.T) {
	for _, modalities := range [][]string{nil, {"text", "audio"}, {"image"}} {
		err := OpenAIRealtimeToGeminiTextEvent(llm.RealtimeEvent{
			Type:     llm.RealtimeEventSessionUpdate,
			WireType: "session.update",
			SessionConfig: &llm.RealtimeSessionConfig{
				OutputModalities: modalities,
			},
		})
		if err == nil {
			t.Fatalf("modalities %#v accepted", modalities)
		}
	}
}

func TestOpenAIRealtimeToGeminiTextEventRejectsNonConversationRole(t *testing.T) {
	err := OpenAIRealtimeToGeminiTextEvent(llm.RealtimeEvent{
		Type:     llm.RealtimeEventItemCreate,
		WireType: "conversation.item.create",
		Message: &llm.Message{
			Role:    llm.RoleDeveloper,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "instruction"}},
		},
	})
	if err == nil {
		t.Fatal("developer realtime item accepted")
	}
}
