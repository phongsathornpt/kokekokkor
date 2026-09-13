package translation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestOpenAIToAnthropicStreamRequestConsumesKnownControls(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		Model:           "portable",
		MaxOutputTokens: &maxTokens,
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}},
		}},
		Metadata: map[string]json.RawMessage{
			"stream":         json.RawMessage("true"),
			"stream_options": json.RawMessage(`{"include_usage":true}`),
			"n":              json.RawMessage("1"),
		},
	}

	translated, options, err := OpenAIToAnthropicStreamRequest(request)
	if err != nil {
		t.Fatalf("OpenAIToAnthropicStreamRequest() error = %v", err)
	}
	if !options.IncludeUsage {
		t.Fatal("IncludeUsage = false, want true")
	}
	if string(translated.Metadata["stream"]) != "true" {
		t.Fatalf("translated stream metadata = %s", translated.Metadata["stream"])
	}
}

func TestOpenAIToAnthropicStreamRequestRejectsUnknownStreamOption(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		MaxOutputTokens: &maxTokens,
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}},
		}},
		Metadata: map[string]json.RawMessage{
			"stream":         json.RawMessage("true"),
			"stream_options": json.RawMessage(`{"mystery":true}`),
		},
	}

	_, _, err := OpenAIToAnthropicStreamRequest(request)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestAnthropicToOpenAIStreamRequestForcesUsage(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		MaxOutputTokens: &maxTokens,
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}},
		}},
		Metadata: map[string]json.RawMessage{"stream": json.RawMessage("true")},
	}

	translated, err := AnthropicToOpenAIStreamRequest(request)
	if err != nil {
		t.Fatalf("AnthropicToOpenAIStreamRequest() error = %v", err)
	}
	if string(translated.Metadata["stream"]) != "true" {
		t.Fatalf("stream metadata = %s", translated.Metadata["stream"])
	}
	if string(translated.Metadata["stream_options"]) != `{"include_usage":true}` {
		t.Fatalf("stream_options = %s", translated.Metadata["stream_options"])
	}
}

func TestStreamEventCompatibilityRejectsLossyUsage(t *testing.T) {
	t.Run("OpenAI reasoning usage to Anthropic", func(t *testing.T) {
		err := OpenAIToAnthropicStreamEvent(llm.StreamEvent{
			Type:  llm.StreamEventUsage,
			Usage: &llm.Usage{ReasoningTokens: 3},
		})
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("error = %v, want ErrUnsupported", err)
		}
	})

	t.Run("Anthropic cache write usage to OpenAI", func(t *testing.T) {
		err := AnthropicToOpenAIStreamEvent(llm.StreamEvent{
			Type:  llm.StreamEventUsage,
			Usage: &llm.Usage{CacheWriteTokens: 7},
		})
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("error = %v, want ErrUnsupported", err)
		}
	})
}
