package translation

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestOpenAIReasoningMapsToAnthropic(t *testing.T) {
	maxTokens := 4096
	request, err := OpenAIToAnthropicRequest(llm.Request{
		Model:           "m",
		MaxOutputTokens: &maxTokens,
		Reasoning:       &llm.ReasoningConfig{Enabled: true, Effort: "minimal"},
	})
	if err != nil {
		t.Fatalf("OpenAIToAnthropicRequest() error = %v", err)
	}
	if request.Reasoning == nil {
		t.Fatal("Reasoning = nil")
	}
	if request.Reasoning.Mode != "adaptive" || request.Reasoning.Effort != "low" || request.Reasoning.Summary != "none" {
		t.Fatalf("Reasoning = %#v", request.Reasoning)
	}
}

func TestOpenAIReasoningMapsToGemini(t *testing.T) {
	request, err := OpenAIToGeminiRequest(llm.Request{
		Model:     "m",
		Reasoning: &llm.ReasoningConfig{Enabled: true, Effort: "xhigh"},
	})
	if err != nil {
		t.Fatalf("OpenAIToGeminiRequest() error = %v", err)
	}
	if request.Reasoning == nil || request.Reasoning.Effort != "high" || request.Reasoning.Summary != "none" {
		t.Fatalf("Reasoning = %#v", request.Reasoning)
	}
}

func TestProviderReasoningIsHiddenFromChatCompletions(t *testing.T) {
	response := llm.Response{
		StopReason: llm.StopReasonEndTurn,
		Content: []llm.ContentBlock{
			llm.ReasoningBlock{Text: "summary", Signature: "provider-secret"},
			llm.TextBlock{Text: "answer"},
		},
	}
	translated, err := GeminiToOpenAIResponse(response)
	if err != nil {
		t.Fatalf("GeminiToOpenAIResponse() error = %v", err)
	}
	if len(translated.Content) != 1 {
		t.Fatalf("Content = %#v", translated.Content)
	}
	text, ok := translated.Content[0].(llm.TextBlock)
	if !ok || text.Text != "answer" {
		t.Fatalf("Content[0] = %#v", translated.Content[0])
	}
}

func TestProviderReasoningBecomesResponsesSummaryWithoutSignature(t *testing.T) {
	response := llm.Response{
		StopReason: llm.StopReasonEndTurn,
		Content: []llm.ContentBlock{
			llm.ReasoningBlock{Text: "summary", Signature: "provider-secret"},
			llm.TextBlock{Text: "answer"},
		},
	}
	translated, err := GeminiToResponsesResponse(response)
	if err != nil {
		t.Fatalf("GeminiToResponsesResponse() error = %v", err)
	}
	if len(translated.Content) != 2 {
		t.Fatalf("Content = %#v", translated.Content)
	}
	reasoning, ok := translated.Content[0].(llm.ReasoningBlock)
	if !ok || reasoning.Text != "summary" || reasoning.Signature != "" || reasoning.RedactedData != "" {
		t.Fatalf("reasoning = %#v", translated.Content[0])
	}
}
