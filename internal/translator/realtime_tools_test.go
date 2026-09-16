package translator

import (
	"strings"
	"testing"
)

func TestRealtimeSessionBridgeFunctionToolRoundTrip(t *testing.T) {
	bridge := NewRealtimeSessionBridge("gemini-live-target")
	setup, err := bridge.ClientMessage([]byte(`{
		"type":"session.update",
		"session":{
			"output_modalities":["text"],
			"tools":[{"type":"function","name":"weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}],
			"tool_choice":"auto"
		}
	}`))
	if err != nil {
		t.Fatalf("setup error = %v", err)
	}
	if len(setup) != 1 || !strings.Contains(string(setup[0]), `"functionDeclarations"`) {
		t.Fatalf("setup = %q", setup)
	}
	if _, err := bridge.ServerMessage([]byte(`{"setupComplete":{}}`)); err != nil {
		t.Fatalf("setupComplete error = %v", err)
	}

	server, err := bridge.ServerMessage([]byte(`{"toolCall":{"functionCalls":[{"id":"call_1","name":"weather","args":{"city":"Bangkok"}}]}}`))
	if err != nil {
		t.Fatalf("toolCall error = %v", err)
	}
	joined := ""
	for _, payload := range server {
		joined += string(payload) + "\n"
	}
	for _, want := range []string{"response.function_call_arguments.delta", "response.function_call_arguments.done", `"call_id":"call_1"`, `"name":"weather"`, "response.done"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("server events missing %q:\n%s", want, joined)
		}
	}

	client, err := bridge.ClientMessage([]byte(`{"type":"conversation.item.create","item":{"type":"function_call_output","call_id":"call_1","output":"{\"temp\":31}"}}`))
	if err != nil {
		t.Fatalf("function_call_output error = %v", err)
	}
	if len(client) != 1 {
		t.Fatalf("client messages = %q", client)
	}
	for _, want := range []string{`"toolResponse"`, `"id":"call_1"`, `"name":"weather"`, `"temp":31`} {
		if !strings.Contains(string(client[0]), want) {
			t.Fatalf("tool response missing %q: %s", want, client[0])
		}
	}

	if _, err := bridge.ClientMessage([]byte(`{"type":"conversation.item.create","item":{"type":"function_call_output","call_id":"call_1","output":"duplicate"}}`)); err == nil {
		t.Fatal("duplicate tool result accepted after correlation was consumed")
	}
}
