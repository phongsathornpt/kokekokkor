package gemini

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestGenerateContentStructuredOutputRoundTrip(t *testing.T) {
	decoded, err := DecodeGenerateContentRequest([]byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"responseFormat":{"text":{"mimeType":"APPLICATION_JSON","schema":{"type":"object","properties":{"ok":{"type":"boolean"}}}}}}}`))
	if err != nil {
		t.Fatalf("DecodeGenerateContentRequest() error = %v", err)
	}
	if decoded.ResponseFormat == nil || !decoded.ResponseFormat.Strict || !json.Valid(decoded.ResponseFormat.JSONSchema) {
		t.Fatalf("response format = %#v", decoded.ResponseFormat)
	}

	encoded, err := EncodeGenerateContentRequest(llm.Request{
		Messages:       []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		ResponseFormat: &llm.ResponseFormat{JSONSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`), Strict: true},
	})
	if err != nil {
		t.Fatalf("EncodeGenerateContentRequest() error = %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	generation := wire["generationConfig"].(map[string]any)
	responseFormat := generation["responseFormat"].(map[string]any)
	text := responseFormat["text"].(map[string]any)
	if text["mimeType"] != "APPLICATION_JSON" || text["schema"] == nil {
		t.Fatalf("responseFormat = %#v", responseFormat)
	}
}
