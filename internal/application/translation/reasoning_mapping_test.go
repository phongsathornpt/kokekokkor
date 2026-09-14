package translation

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestResponsesReasoningSummaryMapsToProviders(t *testing.T) {
	maxTokens := 4096
	request := llm.Request{
		Model:           "m",
		MaxOutputTokens: &maxTokens,
		Reasoning:       &llm.ReasoningConfig{Enabled: true, Effort: "high", Summary: "auto"},
	}

	anthropicRequest, err := ResponsesToAnthropicRequest(request)
	if err != nil {
		t.Fatalf("ResponsesToAnthropicRequest() error = %v", err)
	}
	if anthropicRequest.Reasoning == nil || anthropicRequest.Reasoning.Mode != "adaptive" || anthropicRequest.Reasoning.Summary != "auto" {
		t.Fatalf("Anthropic reasoning = %#v", anthropicRequest.Reasoning)
	}

	geminiRequest, err := ResponsesToGeminiRequest(request)
	if err != nil {
		t.Fatalf("ResponsesToGeminiRequest() error = %v", err)
	}
	if geminiRequest.Reasoning == nil || geminiRequest.Reasoning.Effort != "high" || geminiRequest.Reasoning.Summary != "auto" {
		t.Fatalf("Gemini reasoning = %#v", geminiRequest.Reasoning)
	}
}
