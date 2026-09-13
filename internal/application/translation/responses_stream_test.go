package translation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestResponsesToAnthropicStreamRequestConsumesStreamControls(t *testing.T) {
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
			"stream_options": json.RawMessage(`{"include_obfuscation":false}`),
		},
	}

	translated, options, err := ResponsesToAnthropicStreamRequest(request)
	if err != nil {
		t.Fatalf("ResponsesToAnthropicStreamRequest() error = %v", err)
	}
	if options.IncludeObfuscation {
		t.Fatal("IncludeObfuscation = true, want false")
	}
	if got := string(translated.Metadata["stream"]); got != "true" {
		t.Fatalf("translated stream metadata = %q", got)
	}
	if len(translated.Metadata) != 1 {
		t.Fatalf("translated metadata = %#v", translated.Metadata)
	}
}

func TestResponsesToAnthropicStreamRequestDefaultsObfuscation(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		Model:           "portable",
		MaxOutputTokens: &maxTokens,
		Messages:        []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		Metadata:        map[string]json.RawMessage{"stream": json.RawMessage("true")},
	}

	_, options, err := ResponsesToAnthropicStreamRequest(request)
	if err != nil {
		t.Fatalf("ResponsesToAnthropicStreamRequest() error = %v", err)
	}
	if !options.IncludeObfuscation {
		t.Fatal("IncludeObfuscation = false, want true")
	}
}

func TestResponsesToAnthropicStreamRequestRejectsUnknownStreamOption(t *testing.T) {
	maxTokens := 32
	request := llm.Request{
		Model:           "portable",
		MaxOutputTokens: &maxTokens,
		Messages:        []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		Metadata: map[string]json.RawMessage{
			"stream":         json.RawMessage("true"),
			"stream_options": json.RawMessage(`{"future":true}`),
		},
	}

	_, _, err := ResponsesToAnthropicStreamRequest(request)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}

func TestAnthropicToResponsesStreamEventAllowsPortableUsage(t *testing.T) {
	usage := llm.Usage{InputTokens: 10, OutputTokens: 2, CacheReadTokens: 3, CacheWriteTokens: 4}
	if err := AnthropicToResponsesStreamEvent(llm.StreamEvent{Type: llm.StreamEventUsage, Usage: &usage}); err != nil {
		t.Fatalf("portable usage rejected: %v", err)
	}
}

func TestAnthropicToResponsesStreamEventRejectsThinkingAndRefusal(t *testing.T) {
	for _, event := range []llm.StreamEvent{
		{Type: llm.StreamEventReasoningDelta, ReasoningDelta: "secret"},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonContentBlock},
	} {
		if err := AnthropicToResponsesStreamEvent(event); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("event %q error = %v, want ErrUnsupported", event.Type, err)
		}
	}
}
