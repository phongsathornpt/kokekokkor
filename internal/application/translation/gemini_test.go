package translation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestOpenAIToGeminiRequestAllowsPortableTextAndTools(t *testing.T) {
	request := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: []llm.ContentBlock{llm.TextBlock{Text: "be concise"}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}}},
		},
		Tools: []llm.Tool{{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}},
		Metadata: map[string]json.RawMessage{
			"stream": json.RawMessage("false"),
			"n":      json.RawMessage("1"),
		},
	}
	translated, err := OpenAIToGeminiRequest(request)
	if err != nil {
		t.Fatalf("OpenAIToGeminiRequest() error = %v", err)
	}
	if translated.Metadata != nil {
		t.Fatalf("metadata = %#v", translated.Metadata)
	}
}

func TestOpenAIToGeminiRequestRejectsUnsupportedSemantics(t *testing.T) {
	tests := []struct {
		name    string
		request llm.Request
	}{
		{
			name: "stream",
			request: llm.Request{
				Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
				Metadata: map[string]json.RawMessage{"stream": json.RawMessage("true")},
			},
		},
		{
			name: "developer role",
			request: llm.Request{Messages: []llm.Message{
				{Role: llm.RoleDeveloper, Content: []llm.ContentBlock{llm.TextBlock{Text: "rules"}}},
				{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}},
			}},
		},
		{
			name: "URL image",
			request: llm.Request{Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{
				llm.ImageBlock{Source: llm.MediaSource{Type: llm.MediaSourceURL, URL: "https://example.com/image.png"}},
			}}}},
		},
		{
			name: "structured output",
			request: llm.Request{
				Messages:       []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
				ResponseFormat: &llm.ResponseFormat{JSONSchema: json.RawMessage(`{"type":"object"}`)},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := OpenAIToGeminiRequest(test.request); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("error = %v, want ErrUnsupported", err)
			}
		})
	}
}

func TestGeminiToOpenAIRequestRejectsProviderExtensions(t *testing.T) {
	request := llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		Metadata: map[string]json.RawMessage{"safetySettings": json.RawMessage(`[]`)},
	}
	if _, err := GeminiToOpenAIRequest(request); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error = %v, want ErrUnsupported", err)
	}
}
