package translation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestResponsesToAnthropicAllowsExplicitStatelessFlags(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		MaxOutputTokens: &maxTokens,
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		}},
		Metadata: map[string]json.RawMessage{
			"store":      json.RawMessage("false"),
			"background": json.RawMessage("false"),
		},
	}
	translated, err := ResponsesToAnthropicRequest(request)
	if err != nil {
		t.Fatalf("ResponsesToAnthropicRequest() error = %v", err)
	}
	if len(translated.Metadata) != 0 {
		t.Fatalf("metadata = %#v, want consumed stateless flags", translated.Metadata)
	}
}

func TestResponsesToAnthropicRejectsProviderState(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		MaxOutputTokens: &maxTokens,
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		}},
		Metadata: map[string]json.RawMessage{"store": json.RawMessage("true")},
	}
	_, err := ResponsesToAnthropicRequest(request)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestResponsesToAnthropicRejectsStrictFunctionTool(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		MaxOutputTokens: &maxTokens,
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		}},
		Tools: []llm.Tool{{
			Name:        "lookup",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			Metadata:    map[string]json.RawMessage{"openai.strict": json.RawMessage("true")},
		}},
	}
	_, err := ResponsesToAnthropicRequest(request)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestAnthropicToResponsesAllowsCacheWriteUsage(t *testing.T) {
	response := llm.Response{
		Content:    []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		StopReason: llm.StopReasonEndTurn,
		Usage:      llm.Usage{CacheWriteTokens: 12},
	}
	translated, err := AnthropicToResponsesResponse(response)
	if err != nil {
		t.Fatalf("AnthropicToResponsesResponse() error = %v", err)
	}
	if translated.Usage.CacheWriteTokens != 12 {
		t.Fatalf("cache write tokens = %d", translated.Usage.CacheWriteTokens)
	}
}

func TestAnthropicToResponsesAllowsThinkingSummary(t *testing.T) {
	translated, err := AnthropicToResponsesResponse(llm.Response{
		Content:    []llm.ContentBlock{llm.ReasoningBlock{Text: "summary", Signature: "anthropic-signature"}},
		StopReason: llm.StopReasonEndTurn,
	})
	if err != nil {
		t.Fatalf("AnthropicToResponsesResponse() error = %v", err)
	}
	if len(translated.Content) != 1 {
		t.Fatalf("content = %#v", translated.Content)
	}
	reasoning, ok := translated.Content[0].(llm.ReasoningBlock)
	if !ok || reasoning.Text != "summary" || reasoning.Signature != "" {
		t.Fatalf("reasoning = %#v", translated.Content[0])
	}
}

func TestResponsesToGeminiAllowsExplicitStatelessFlags(t *testing.T) {
	request := llm.Request{
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		}},
		Metadata: map[string]json.RawMessage{
			"store":      json.RawMessage("false"),
			"background": json.RawMessage("false"),
		},
	}
	translated, err := ResponsesToGeminiRequest(request)
	if err != nil {
		t.Fatalf("ResponsesToGeminiRequest() error = %v", err)
	}
	if len(translated.Metadata) != 0 {
		t.Fatalf("metadata = %#v, want consumed stateless flags", translated.Metadata)
	}
}

func TestResponsesToGeminiRejectsBackgroundExecution(t *testing.T) {
	request := llm.Request{
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		}},
		Metadata: map[string]json.RawMessage{"background": json.RawMessage("true")},
	}
	_, err := ResponsesToGeminiRequest(request)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}
