package translation

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestStructuredOutputPortableRequestPaths(t *testing.T) {
	maxTokens := 64
	base := llm.Request{
		MaxOutputTokens: &maxTokens,
		Messages:        []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		ResponseFormat: &llm.ResponseFormat{
			Name:       "answer",
			JSONSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"]}`),
			Strict:     true,
		},
	}
	tests := []struct {
		name string
		fn   func(llm.Request) (llm.Request, error)
	}{
		{"openai to anthropic", OpenAIToAnthropicRequest},
		{"anthropic to openai", AnthropicToOpenAIRequest},
		{"openai to gemini", OpenAIToGeminiRequest},
		{"gemini to openai", GeminiToOpenAIRequest},
		{"responses to anthropic", ResponsesToAnthropicRequest},
		{"responses to gemini", ResponsesToGeminiRequest},
		{"anthropic to gemini", AnthropicToGeminiRequest},
		{"gemini to anthropic", GeminiToAnthropicRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			translated, err := test.fn(base)
			if err != nil {
				t.Fatalf("translation error = %v", err)
			}
			if translated.ResponseFormat == nil || !json.Valid(translated.ResponseFormat.JSONSchema) {
				t.Fatalf("response format = %#v", translated.ResponseFormat)
			}
		})
	}
}
