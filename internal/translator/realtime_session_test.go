package translator

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestRealtimeSessionBridgeRequiresSetupAndAck(t *testing.T) {
	bridge := NewRealtimeSessionBridge("gemini-live-target")

	if _, err := bridge.ClientMessage([]byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`)); !errors.Is(err, ErrRealtimeSetupRequired) {
		t.Fatalf("content before setup error = %v, want %v", err, ErrRealtimeSetupRequired)
	}

	messages, err := bridge.ClientMessage([]byte(`{"type":"session.update","session":{"model":"source-model","instructions":"be concise","output_modalities":["text"]}}`))
	if err != nil {
		t.Fatalf("session update error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("setup message count = %d", len(messages))
	}
	var setup map[string]json.RawMessage
	if err := json.Unmarshal(messages[0], &setup); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	if _, ok := setup["setup"]; !ok {
		t.Fatalf("Gemini setup payload = %s", messages[0])
	}

	if _, err := bridge.ClientMessage([]byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`)); !errors.Is(err, ErrRealtimeSetupPending) {
		t.Fatalf("content before setupComplete error = %v, want %v", err, ErrRealtimeSetupPending)
	}

	messages, err = bridge.ServerMessage([]byte(`{"setupComplete":{}}`))
	if err != nil {
		t.Fatalf("setupComplete error = %v", err)
	}
	if len(messages) != 0 || !bridge.SetupComplete() {
		t.Fatalf("setupComplete outputs = %d, state = %v", len(messages), bridge.SetupComplete())
	}

	messages, err = bridge.ClientMessage([]byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`))
	if err != nil {
		t.Fatalf("item create error = %v", err)
	}
	if len(messages) != 1 || !json.Valid(messages[0]) {
		t.Fatalf("Gemini clientContent = %q", messages)
	}

	messages, err = bridge.ClientMessage([]byte(`{"type":"response.create"}`))
	if err != nil {
		t.Fatalf("response create error = %v", err)
	}
	if len(messages) != 1 || string(messages[0]) != `{"clientContent":{"turnComplete":true}}` {
		t.Fatalf("turnComplete payload = %q", messages)
	}
}

func TestRealtimeSessionBridgeTranslatesGeminiTextOutput(t *testing.T) {
	bridge := NewRealtimeSessionBridge("gemini-live-target")
	if _, err := bridge.ClientMessage([]byte(`{"type":"session.update","session":{"output_modalities":["text"]}}`)); err != nil {
		t.Fatalf("setup error = %v", err)
	}
	if _, err := bridge.ServerMessage([]byte(`{"setupComplete":{}}`)); err != nil {
		t.Fatalf("setupComplete error = %v", err)
	}

	messages, err := bridge.ServerMessage([]byte(`{"serverContent":{"modelTurn":{"parts":[{"text":"hello"}]}}}`))
	if err != nil {
		t.Fatalf("text output error = %v", err)
	}
	var types []string
	for _, payload := range messages {
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode OpenAI event %s: %v", payload, err)
		}
		types = append(types, event.Type)
	}
	want := []string{"response.created", "response.output_item.added", "response.content_part.added", "response.output_text.delta"}
	if len(types) != len(want) {
		t.Fatalf("event types = %#v", types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("types[%d] = %q, want %q", i, types[i], want[i])
		}
	}

	messages, err = bridge.ServerMessage([]byte(`{"serverContent":{"turnComplete":true},"usageMetadata":{"promptTokenCount":4,"responseTokenCount":1}}`))
	if err != nil {
		t.Fatalf("turn complete error = %v", err)
	}
	types = types[:0]
	for _, payload := range messages {
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("decode OpenAI completion event: %v", err)
		}
		types = append(types, event.Type)
	}
	want = []string{"response.output_text.done", "response.content_part.done", "response.output_item.done", "response.done"}
	if len(types) != len(want) {
		t.Fatalf("completion types = %#v", types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("completion types[%d] = %q, want %q", i, types[i], want[i])
		}
	}
}

func TestRealtimeSessionBridgeRejectsProviderControlState(t *testing.T) {
	bridge := NewRealtimeSessionBridge("gemini-live-target")
	if _, err := bridge.ClientMessage([]byte(`{"type":"session.update","session":{"output_modalities":["text"]}}`)); err != nil {
		t.Fatalf("setup error = %v", err)
	}
	if _, err := bridge.ServerMessage([]byte(`{"setupComplete":{}}`)); err != nil {
		t.Fatalf("setupComplete error = %v", err)
	}
	if _, err := bridge.ServerMessage([]byte(`{"goAway":{"timeLeft":"10s"}}`)); err == nil {
		t.Fatal("goAway accepted")
	}
	if _, err := bridge.ServerMessage([]byte(`{"sessionResumptionUpdate":{"resumable":true}}`)); err == nil {
		t.Fatal("session resumption state accepted")
	}
}
