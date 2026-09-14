package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestMessagesStructuredOutputRoundTrip(t *testing.T) {
	decoded, err := DecodeMessagesRequest([]byte(`{"model":"claude","max_tokens":64,"messages":[{"role":"user","content":"hi"}],"output_config":{"effort":"high","format":{"type":"json_schema","schema":{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"]}}}}`))
	if err != nil {
		t.Fatalf("DecodeMessagesRequest() error = %v", err)
	}
	if decoded.ResponseFormat == nil || !decoded.ResponseFormat.Strict || decoded.Reasoning == nil || decoded.Reasoning.Effort != "high" {
		t.Fatalf("decoded controls = format %#v reasoning %#v", decoded.ResponseFormat, decoded.Reasoning)
	}

	maxTokens := 64
	encoded, err := EncodeMessagesRequest(llm.Request{
		Model:           "claude",
		MaxOutputTokens: &maxTokens,
		Messages:        []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		Reasoning:       &llm.ReasoningConfig{Enabled: true, Mode: "adaptive", Effort: "high"},
		ResponseFormat:  &llm.ResponseFormat{JSONSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`), Strict: true},
	})
	if err != nil {
		t.Fatalf("EncodeMessagesRequest() error = %v", err)
	}
	var wire struct {
		OutputConfig struct {
			Effort string `json:"effort"`
			Format struct {
				Type   string          `json:"type"`
				Schema json.RawMessage `json:"schema"`
			} `json:"format"`
		} `json:"output_config"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.OutputConfig.Effort != "high" || wire.OutputConfig.Format.Type != "json_schema" || !json.Valid(wire.OutputConfig.Format.Schema) {
		t.Fatalf("output_config = %#v", wire.OutputConfig)
	}
}
