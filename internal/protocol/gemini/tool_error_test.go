package gemini

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestEncodeToolResultResponsePreservesError(t *testing.T) {
	result := llm.ToolResultBlock{
		ToolCallID: "call-1",
		IsError:    true,
		Content:    []llm.ContentBlock{llm.TextBlock{Text: "boom"}},
	}

	raw, err := encodeToolResultResponse(result)
	if err != nil {
		t.Fatalf("encodeToolResultResponse() error = %v", err)
	}
	var object map[string]string
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := object["error"]; got != "boom" {
		t.Fatalf("error = %q", got)
	}
}

func TestDecodeGeminiPartsMarksErrorResponse(t *testing.T) {
	parts := []geminiPart{{
		FunctionResponse: &geminiFunctionResponse{
			ID:       "call-1",
			Name:     "lookup",
			Response: json.RawMessage(`{"error":"boom"}`),
		},
	}}

	blocks, _, err := decodeGeminiParts(parts, 0, nil)
	if err != nil {
		t.Fatalf("decodeGeminiParts() error = %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("len(blocks) = %d", len(blocks))
	}
	result, ok := blocks[0].(llm.ToolResultBlock)
	if !ok {
		t.Fatalf("block type = %T", blocks[0])
	}
	if !result.IsError {
		t.Fatal("IsError = false")
	}
	if len(result.Content) != 1 {
		t.Fatalf("len(content) = %d", len(result.Content))
	}
	text, ok := result.Content[0].(llm.TextBlock)
	if !ok || text.Text != "boom" {
		t.Fatalf("content = %#v", result.Content[0])
	}
}
