package translation

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestPortableStreamCompatibilityMatrix(t *testing.T) {
	validators := []struct {
		name string
		fn   func(llm.StreamEvent) error
	}{
		{"openai_to_anthropic", OpenAIToAnthropicStreamEvent},
		{"anthropic_to_openai", AnthropicToOpenAIStreamEvent},
		{"openai_to_gemini", OpenAIToGeminiStreamEvent},
		{"gemini_to_openai", GeminiToOpenAIStreamEvent},
		{"anthropic_to_gemini", AnthropicToGeminiStreamEvent},
		{"gemini_to_anthropic", GeminiToAnthropicStreamEvent},
		{"anthropic_to_responses", AnthropicToResponsesStreamEvent},
		{"gemini_to_responses", GeminiToResponsesStreamEvent},
	}

	events := []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, ResponseID: "resp_1", Model: "portable-model"},
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}},
		{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: "hello"},
		{Type: llm.StreamEventContentStop, Index: 0},
		{Type: llm.StreamEventUsage, Usage: &llm.Usage{InputTokens: 3, OutputTokens: 1}},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonEndTurn},
	}

	for _, validator := range validators {
		t.Run(validator.name, func(t *testing.T) {
			for _, event := range events {
				if err := validator.fn(event); err != nil {
					t.Fatalf("event %q rejected: %v", event.Type, err)
				}
			}
		})
	}
}
