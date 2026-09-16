package gemini

import (
	"encoding/json"
	"testing"
)

func TestDecodeLiveServerMessageSetupComplete(t *testing.T) {
	message, err := DecodeLiveServerMessage([]byte(`{"setupComplete":{}}`))
	if err != nil {
		t.Fatalf("DecodeLiveServerMessage() error = %v", err)
	}
	if !message.SetupComplete {
		t.Fatal("SetupComplete = false")
	}
}

func TestDecodeLiveServerMessageTextTurn(t *testing.T) {
	message, err := DecodeLiveServerMessage([]byte(`{
		"serverContent": {
			"modelTurn": {"parts": [{"text":"hello"},{"text":" world"}]},
			"generationComplete": true,
			"turnComplete": true
		},
		"usageMetadata": {
			"promptTokenCount": 10,
			"responseTokenCount": 4,
			"thoughtsTokenCount": 2,
			"cachedContentTokenCount": 3
		}
	}`))
	if err != nil {
		t.Fatalf("DecodeLiveServerMessage() error = %v", err)
	}
	if message.ServerContent == nil {
		t.Fatal("ServerContent = nil")
	}
	if len(message.ServerContent.Text) != 2 || message.ServerContent.Text[0] != "hello" || message.ServerContent.Text[1] != " world" {
		t.Fatalf("Text = %#v", message.ServerContent.Text)
	}
	if !message.ServerContent.GenerationComplete || !message.ServerContent.TurnComplete {
		t.Fatalf("completion flags = %#v", message.ServerContent)
	}
	if message.Usage == nil {
		t.Fatal("Usage = nil")
	}
	if message.Usage.InputTokens != 10 || message.Usage.OutputTokens != 4 || message.Usage.ReasoningTokens != 2 || message.Usage.CacheReadTokens != 3 {
		t.Fatalf("Usage = %#v", message.Usage)
	}
}

func TestDecodeLiveServerMessageInterruptedAndMetadata(t *testing.T) {
	message, err := DecodeLiveServerMessage([]byte(`{
		"serverContent": {
			"interrupted": true,
			"providerField": {"x":1}
		},
		"futureTopLevel": true
	}`))
	if err != nil {
		t.Fatalf("DecodeLiveServerMessage() error = %v", err)
	}
	if message.ServerContent == nil || !message.ServerContent.Interrupted {
		t.Fatalf("ServerContent = %#v", message.ServerContent)
	}
	if _, ok := message.ServerContent.Metadata["providerField"]; !ok {
		t.Fatal("serverContent provider field was discarded")
	}
	if _, ok := message.Metadata["futureTopLevel"]; !ok {
		t.Fatal("top-level provider field was discarded")
	}
}

func TestDecodeLiveServerMessagePreservesToolCall(t *testing.T) {
	message, err := DecodeLiveServerMessage([]byte(`{"toolCall":{"functionCalls":[{"id":"call_1","name":"weather","args":{"city":"Bangkok"}}]}}`))
	if err != nil {
		t.Fatalf("DecodeLiveServerMessage() error = %v", err)
	}
	if !json.Valid(message.ToolCall) {
		t.Fatalf("ToolCall = %q", message.ToolCall)
	}
}

func TestDecodeLiveServerMessageRejectsMultipleMessageTypes(t *testing.T) {
	_, err := DecodeLiveServerMessage([]byte(`{"setupComplete":{},"serverContent":{"turnComplete":true}}`))
	if err == nil {
		t.Fatal("multiple messageType fields accepted")
	}
}
