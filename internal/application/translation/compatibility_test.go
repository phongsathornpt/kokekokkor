package translation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestOpenAIToAnthropicRejectsStreaming(t *testing.T) {
	maxTokens := 32
	_, err := OpenAIToAnthropicRequest(llm.Request{
		Model:           "m",
		MaxOutputTokens: &maxTokens,
		Metadata: map[string]json.RawMessage{
			"stream": json.RawMessage(`true`),
		},
	})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestOpenAIToAnthropicConsumesNonStreamingControls(t *testing.T) {
	maxTokens := 32
	request, err := OpenAIToAnthropicRequest(llm.Request{
		Model:           "m",
		MaxOutputTokens: &maxTokens,
		Metadata: map[string]json.RawMessage{
			"stream": json.RawMessage(`false`),
			"n":      json.RawMessage(`1`),
		},
	})
	if err != nil {
		t.Fatalf("OpenAIToAnthropicRequest() error = %v", err)
	}
	if len(request.Metadata) != 0 {
		t.Fatalf("metadata = %#v, want empty", request.Metadata)
	}
}

func TestOpenAIToAnthropicRejectsUnknownProviderExtension(t *testing.T) {
	maxTokens := 32
	_, err := OpenAIToAnthropicRequest(llm.Request{
		Model:           "m",
		MaxOutputTokens: &maxTokens,
		Metadata: map[string]json.RawMessage{
			"logprobs": json.RawMessage(`true`),
		},
	})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestAnthropicToOpenAIRejectsThinkingAndErrorToolResults(t *testing.T) {
	t.Run("thinking", func(t *testing.T) {
		_, err := AnthropicToOpenAIRequest(llm.Request{
			Model:     "m",
			Reasoning: &llm.ReasoningConfig{Enabled: true, Mode: "adaptive"},
		})
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("error = %v, want ErrUnsupported", err)
		}
	})

	t.Run("tool error flag", func(t *testing.T) {
		_, err := AnthropicToOpenAIRequest(llm.Request{
			Model: "m",
			Messages: []llm.Message{{
				Role: llm.RoleUser,
				Content: []llm.ContentBlock{llm.ToolResultBlock{
					ToolCallID: "tool_1",
					IsError:    true,
					Content:    []llm.ContentBlock{llm.TextBlock{Text: "failed"}},
				}},
			}},
		})
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("error = %v, want ErrUnsupported", err)
		}
	})
}

func TestAnthropicToOpenAIResponseRejectsCacheWriteUsage(t *testing.T) {
	_, err := AnthropicToOpenAIResponse(llm.Response{
		StopReason: llm.StopReasonEndTurn,
		Usage:      llm.Usage{CacheWriteTokens: 3},
	})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}
