package translation

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestPortableRequestCompatibilityMatrix(t *testing.T) {
	maxTokens := 64
	base := llm.Request{
		Model: "portable-model",
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		}},
		MaxOutputTokens: &maxTokens,
	}

	cases := []struct {
		name string
		fn   func(llm.Request) (llm.Request, error)
	}{
		{"openai_to_anthropic", OpenAIToAnthropicRequest},
		{"anthropic_to_openai", AnthropicToOpenAIRequest},
		{"openai_to_gemini", OpenAIToGeminiRequest},
		{"gemini_to_openai", GeminiToOpenAIRequest},
		{"responses_to_anthropic", ResponsesToAnthropicRequest},
		{"responses_to_gemini", ResponsesToGeminiRequest},
		{"anthropic_to_gemini", AnthropicToGeminiRequest},
		{"gemini_to_anthropic", GeminiToAnthropicRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.fn(base)
			if err != nil {
				t.Fatalf("translate portable request: %v", err)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("translated request invalid: %v", err)
			}
			if got.Model != base.Model || len(got.Messages) != 1 {
				t.Fatalf("translated request = %#v", got)
			}
			text, ok := got.Messages[0].Content[0].(llm.TextBlock)
			if !ok || text.Text != "hello" {
				t.Fatalf("translated content = %#v", got.Messages[0].Content)
			}
		})
	}
}
