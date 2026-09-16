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

func TestDecodeRealtimeEventCanonicalSession(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{
		"type":"session.update",
		"session":{
			"model":"gpt-realtime",
			"instructions":"be concise",
			"output_modalities":["text"]
		}
	}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.SessionConfig == nil {
		t.Fatal("SessionConfig = nil")
	}
	if event.SessionConfig.Model != "gpt-realtime" || event.SessionConfig.Instructions != "be concise" {
		t.Fatalf("SessionConfig = %#v", event.SessionConfig)
	}
	if len(event.SessionConfig.OutputModalities) != 1 || event.SessionConfig.OutputModalities[0] != "text" {
		t.Fatalf("OutputModalities = %#v", event.SessionConfig.OutputModalities)
	}
}

func TestDecodeRealtimeEventLegacyModalities(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{"type":"session.update","session":{"modalities":["text"]}}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.SessionConfig == nil || len(event.SessionConfig.OutputModalities) != 1 || event.SessionConfig.OutputModalities[0] != "text" {
		t.Fatalf("SessionConfig = %#v", event.SessionConfig)
	}
}

func TestDecodeRealtimeEventCanonicalMessage(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{
		"type":"conversation.item.create",
		"item":{
			"type":"message",
			"role":"user",
			"content":[{"type":"input_text","text":"hello"}]
		}
	}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.Message == nil {
		t.Fatal("Message = nil")
	}
	if event.Message.Role != llm.RoleUser {
		t.Fatalf("Role = %q", event.Message.Role)
	}
	if len(event.Message.Content) != 1 {
		t.Fatalf("Content len = %d", len(event.Message.Content))
	}
	text, ok := event.Message.Content[0].(llm.TextBlock)
	if !ok || text.Text != "hello" {
		t.Fatalf("Content[0] = %#v", event.Message.Content[0])
	}
}

func TestDecodeRealtimeEventNonTextItemRemainsNonPortable(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{
		"type":"conversation.item.create",
		"item":{"type":"message","role":"user","content":[{"type":"input_audio","audio":"YWJj"}]}
	}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.Message != nil {
		t.Fatalf("Message = %#v, want nil", event.Message)
	}
	if len(event.Item) == 0 {
		t.Fatal("raw item was discarded")
	}
}

func TestDecodeRealtimeEventPreservesProviderFields(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{"type":"session.update","event_id":"evt_1","session":{"instructions":"hello","provider_session_field":true},"provider_extension":{"x":1}}`))
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
	if event.SessionConfig == nil {
		t.Fatal("SessionConfig = nil")
	}
	if _, ok := event.SessionConfig.Metadata["provider_session_field"]; !ok {
		t.Fatal("provider session field was discarded")
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
