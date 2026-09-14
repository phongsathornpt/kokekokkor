package translation

import (
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestResponsesToGeminiNormalizesLeadingInstructions(t *testing.T) {
	request := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleDeveloper, Content: []llm.ContentBlock{llm.TextBlock{Text: "be concise"}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}}},
		},
	}

	got, err := ResponsesToGeminiRequest(request)
	if err != nil {
		t.Fatalf("ResponsesToGeminiRequest() error = %v", err)
	}
	if len(got.Messages) != 2 || got.Messages[0].Role != llm.RoleSystem || got.Messages[1].Role != llm.RoleUser {
		t.Fatalf("normalized messages = %#v", got.Messages)
	}
	text, ok := got.Messages[0].Content[0].(llm.TextBlock)
	if !ok || text.Text != "be concise" {
		t.Fatalf("system instruction = %#v", got.Messages[0].Content)
	}
}

func TestResponsesToGeminiRejectsMixedInstructionPrecedence(t *testing.T) {
	request := llm.Request{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: []llm.ContentBlock{llm.TextBlock{Text: "system"}}},
		{Role: llm.RoleDeveloper, Content: []llm.ContentBlock{llm.TextBlock{Text: "developer"}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}}},
	}}

	_, err := ResponsesToGeminiRequest(request)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("ResponsesToGeminiRequest() error = %v, want unsupported", err)
	}
}

func TestGeminiToResponsesResponseAllowsReasoningUsageAndSummary(t *testing.T) {
	response := llm.Response{
		Content:    []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		StopReason: llm.StopReasonEndTurn,
		Usage:      llm.Usage{ReasoningTokens: 7},
	}
	if _, err := GeminiToResponsesResponse(response); err != nil {
		t.Fatalf("GeminiToResponsesResponse() error = %v", err)
	}

	response.Content = []llm.ContentBlock{llm.ReasoningBlock{Text: "thought", Signature: "gemini-signature"}}
	translated, err := GeminiToResponsesResponse(response)
	if err != nil {
		t.Fatalf("GeminiToResponsesResponse() reasoning error = %v", err)
	}
	if len(translated.Content) != 1 {
		t.Fatalf("content = %#v", translated.Content)
	}
	reasoning, ok := translated.Content[0].(llm.ReasoningBlock)
	if !ok || reasoning.Text != "thought" || reasoning.Signature != "" {
		t.Fatalf("reasoning = %#v", translated.Content[0])
	}
}
