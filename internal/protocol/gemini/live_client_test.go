package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestLiveClientEncoderSetupUsesTargetModel(t *testing.T) {
	encoder := NewLiveClientEncoder("gemini-live-target")
	payload, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventSessionUpdate,
		SessionConfig: &llm.RealtimeSessionConfig{
			Model:            "source-model",
			Instructions:     "be concise",
			OutputModalities: []string{"text"},
		},
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var got struct {
		Setup struct {
			Model              string   `json:"model"`
			ResponseModalities []string `json:"responseModalities"`
			SystemInstruction  struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"systemInstruction"`
		} `json:"setup"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Setup.Model != "models/gemini-live-target" {
		t.Fatalf("model = %q", got.Setup.Model)
	}
	if len(got.Setup.ResponseModalities) != 1 || got.Setup.ResponseModalities[0] != "TEXT" {
		t.Fatalf("responseModalities = %#v", got.Setup.ResponseModalities)
	}
	if len(got.Setup.SystemInstruction.Parts) != 1 || got.Setup.SystemInstruction.Parts[0].Text != "be concise" {
		t.Fatalf("systemInstruction = %#v", got.Setup.SystemInstruction)
	}
}

func TestLiveClientEncoderMessageAndTurnComplete(t *testing.T) {
	encoder := NewLiveClientEncoder("models/gemini-live-target")
	if _, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventSessionStart,
		SessionConfig: &llm.RealtimeSessionConfig{
			OutputModalities: []string{"text"},
		},
	}); err != nil {
		t.Fatalf("setup Encode() error = %v", err)
	}

	payload, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventItemCreate,
		Message: &llm.Message{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{
				llm.TextBlock{Text: "hello"},
				llm.TextBlock{Text: "world"},
			},
		},
	})
	if err != nil {
		t.Fatalf("message Encode() error = %v", err)
	}
	if !strings.Contains(string(payload), `"role":"user"`) || !strings.Contains(string(payload), `"turnComplete":false`) {
		t.Fatalf("message payload = %s", payload)
	}
	if !strings.Contains(string(payload), `"text":"hello"`) || !strings.Contains(string(payload), `"text":"world"`) {
		t.Fatalf("message payload = %s", payload)
	}

	payload, err = encoder.Encode(llm.RealtimeEvent{Type: llm.RealtimeEventResponseCreate})
	if err != nil {
		t.Fatalf("response Encode() error = %v", err)
	}
	if string(payload) != `{"clientContent":{"turnComplete":true}}` {
		t.Fatalf("turn complete payload = %s", payload)
	}
}

func TestLiveClientEncoderAssistantRoleMapsToModel(t *testing.T) {
	encoder := NewLiveClientEncoder("gemini-live")
	_, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventSessionUpdate,
		SessionConfig: &llm.RealtimeSessionConfig{OutputModalities: []string{"text"}},
	})
	if err != nil {
		t.Fatalf("setup Encode() error = %v", err)
	}
	payload, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventItemCreate,
		Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock{Text: "prior answer"}}},
	})
	if err != nil {
		t.Fatalf("message Encode() error = %v", err)
	}
	if !strings.Contains(string(payload), `"role":"model"`) {
		t.Fatalf("payload = %s", payload)
	}
}

func TestLiveClientEncoderRejectsInvalidOrderingAndShapes(t *testing.T) {
	encoder := NewLiveClientEncoder("gemini-live")
	if _, err := encoder.Encode(llm.RealtimeEvent{Type: llm.RealtimeEventResponseCreate}); err == nil {
		t.Fatal("response before setup accepted")
	}
	if _, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventSessionUpdate,
		SessionConfig: &llm.RealtimeSessionConfig{OutputModalities: []string{"text"}},
	}); err != nil {
		t.Fatalf("setup Encode() error = %v", err)
	}
	if _, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventSessionUpdate,
		SessionConfig: &llm.RealtimeSessionConfig{OutputModalities: []string{"text"}},
	}); err == nil {
		t.Fatal("second setup accepted")
	}
	if _, err := encoder.Encode(llm.RealtimeEvent{
		Type: llm.RealtimeEventItemCreate,
		Message: &llm.Message{Role: llm.RoleSystem, Content: []llm.ContentBlock{llm.TextBlock{Text: "nope"}}},
	}); err == nil {
		t.Fatal("system-role client content accepted")
	}
}
