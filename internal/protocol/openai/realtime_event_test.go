package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeRealtimeEventPortableControls(t *testing.T) {
	tests := []struct {
		name string
		body string
		want llm.RealtimeEventType
	}{
		{name: "session start", body: `{"type":"session.start","session":{"model":"gpt-realtime"}}`, want: llm.RealtimeEventSessionStart},
		{name: "session update", body: `{"type":"session.update","session":{"instructions":"be concise"}}`, want: llm.RealtimeEventSessionUpdate},
		{name: "legacy audio append", body: `{"type":"input_audio_buffer.append","audio":"YWJj"}`, want: llm.RealtimeEventInputAudioAppend},
		{name: "live audio append", body: `{"type":"input_audio.append","audio":"YWJj"}`, want: llm.RealtimeEventInputAudioAppend},
		{name: "legacy item create", body: `{"type":"conversation.item.create","item":{"type":"message"}}`, want: llm.RealtimeEventItemCreate},
		{name: "live item create", body: `{"type":"response.item.create","item":{"type":"message"}}`, want: llm.RealtimeEventItemCreate},
		{name: "response create", body: `{"type":"response.create"}`, want: llm.RealtimeEventResponseCreate},
		{name: "response cancel", body: `{"type":"response.cancel"}`, want: llm.RealtimeEventResponseCancel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := DecodeRealtimeEvent([]byte(tt.body))
			if err != nil {
				t.Fatalf("DecodeRealtimeEvent() error = %v", err)
			}
			if event.Type != tt.want {
				t.Fatalf("Type = %q, want %q", event.Type, tt.want)
			}
		})
	}
}

func TestDecodeRealtimeEventPreservesProviderFields(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{"type":"session.update","event_id":"evt_1","session":{"instructions":"hello"},"provider_extension":{"x":1}}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.EventID != "evt_1" {
		t.Fatalf("EventID = %q, want evt_1", event.EventID)
	}
	if event.WireType != "session.update" {
		t.Fatalf("WireType = %q", event.WireType)
	}
	if !json.Valid(event.Session) {
		t.Fatalf("Session is not valid JSON: %q", event.Session)
	}
	if _, ok := event.Metadata["provider_extension"]; !ok {
		t.Fatal("provider_extension metadata was discarded")
	}
}

func TestDecodeRealtimeEventUnknownTypeIsExplicit(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{"type":"vendor.future.event","foo":"bar"}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.Type != llm.RealtimeEventUnknown {
		t.Fatalf("Type = %q, want %q", event.Type, llm.RealtimeEventUnknown)
	}
	if event.WireType != "vendor.future.event" {
		t.Fatalf("WireType = %q", event.WireType)
	}
	if _, ok := event.Metadata["foo"]; !ok {
		t.Fatal("unknown provider field was discarded")
	}
}

func TestDecodeRealtimeEventRejectsMissingType(t *testing.T) {
	if _, err := DecodeRealtimeEvent([]byte(`{"event_id":"evt_1"}`)); err == nil {
		t.Fatal("DecodeRealtimeEvent() error = nil, want error")
	}
}
