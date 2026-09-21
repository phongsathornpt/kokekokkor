package translation

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestAnthropicToOpenAIRequestPreservesDisableParallel(t *testing.T) {
	request := llm.Request{
		Model:      "m",
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		ToolChoice: &llm.ToolChoice{Mode: llm.ToolChoiceAuto, DisableParallel: true},
	}

	translated, err := AnthropicToOpenAIRequest(request)
	if err != nil {
		t.Fatalf("AnthropicToOpenAIRequest() error = %v", err)
	}
	if translated.ToolChoice == nil || !translated.ToolChoice.DisableParallel {
		t.Fatalf("tool choice = %#v", translated.ToolChoice)
	}
}
