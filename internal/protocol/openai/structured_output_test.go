package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeResponsesRequestStructuredOutput(t *testing.T) {
	request, err := DecodeResponsesRequest([]byte(`{"model":"gpt","input":"hi","text":{"format":{"type":"json_schema","name":"answer","description":"answer shape","schema":{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false},"strict":true}}}`))
	if err != nil {
		t.Fatalf("DecodeResponsesRequest() error = %v", err)
	}
	if request.ResponseFormat == nil || request.ResponseFormat.Name != "answer" || request.ResponseFormat.Description != "answer shape" || !request.ResponseFormat.Strict {
		t.Fatalf("response format = %#v", request.ResponseFormat)
	}
	if !json.Valid(request.ResponseFormat.JSONSchema) {
		t.Fatalf("schema = %s", request.ResponseFormat.JSONSchema)
	}
}

func TestEncodeChatResponseFormatSynthesizesName(t *testing.T) {
	raw, err := encodeChatResponseFormat(&llm.ResponseFormat{JSONSchema: json.RawMessage(`{"type":"object"}`), Strict: true})
	if err != nil {
		t.Fatalf("encodeChatResponseFormat() error = %v", err)
	}
	var wire struct {
		JSONSchema struct {
			Name string `json:"name"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.JSONSchema.Name != "structured_output" {
		t.Fatalf("name = %q", wire.JSONSchema.Name)
	}
}
