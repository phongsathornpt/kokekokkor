package translation

import (
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestAnthropicToGeminiRequestAllowsToolErrors(t *testing.T) {
	request := llm.Request{
		Model: "m",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{llm.ToolResultBlock{
				ToolCallID: "call-1",
				IsError:    true,
				Content:    []llm.ContentBlock{llm.TextBlock{Text: "boom"}},
			}},
		}},
	}

	translated, err := AnthropicToGeminiRequest(request)
	if err != nil {
		t.Fatalf("AnthropicToGeminiRequest() error = %v", err)
	}
	result, ok := translated.Messages[0].Content[0].(llm.ToolResultBlock)
	if !ok || !result.IsError {
		t.Fatalf("tool result = %#v", translated.Messages[0].Content[0])
	}
}

func TestGeminiToAnthropicRequestAllowsToolErrors(t *testing.T) {
	request := llm.Request{
		Model: "m",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{llm.ToolResultBlock{
				ToolCallID: "call-1",
				IsError:    true,
				Content:    []llm.ContentBlock{llm.TextBlock{Text: "boom"}},
			}},
		}},
	}

	translated, err := GeminiToAnthropicRequest(request)
	if err != nil {
		t.Fatalf("GeminiToAnthropicRequest() error = %v", err)
	}
	result, ok := translated.Messages[0].Content[0].(llm.ToolResultBlock)
	if !ok || !result.IsError {
		t.Fatalf("tool result = %#v", translated.Messages[0].Content[0])
	}
}
