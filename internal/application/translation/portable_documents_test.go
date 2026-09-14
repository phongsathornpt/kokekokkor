package translation

import (
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestAnthropicToGeminiRequestAllowsBase64Documents(t *testing.T) {
	request := llm.Request{
		Model: "m",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{llm.DocumentBlock{Source: llm.MediaSource{
				Type:      llm.MediaSourceBase64,
				MediaType: "application/pdf",
				Data:      "cGRm",
			}}},
		}},
	}

	translated, err := AnthropicToGeminiRequest(request)
	if err != nil {
		t.Fatalf("AnthropicToGeminiRequest() error = %v", err)
	}
	document, ok := translated.Messages[0].Content[0].(llm.DocumentBlock)
	if !ok {
		t.Fatalf("content = %#v", translated.Messages[0].Content)
	}
	if document.Source.Type != llm.MediaSourceBase64 || document.Source.MediaType != "application/pdf" || document.Source.Data != "cGRm" {
		t.Fatalf("document = %#v", document)
	}
}

func TestGeminiToAnthropicRequestAllowsBase64Documents(t *testing.T) {
	request := llm.Request{
		Model: "m",
		Messages: []llm.Message{{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{llm.DocumentBlock{Source: llm.MediaSource{
				Type:      llm.MediaSourceBase64,
				MediaType: "text/plain",
				Data:      "aGVsbG8=",
			}}},
		}},
	}

	translated, err := GeminiToAnthropicRequest(request)
	if err != nil {
		t.Fatalf("GeminiToAnthropicRequest() error = %v", err)
	}
	document, ok := translated.Messages[0].Content[0].(llm.DocumentBlock)
	if !ok {
		t.Fatalf("content = %#v", translated.Messages[0].Content)
	}
	if document.Source.Type != llm.MediaSourceBase64 || document.Source.MediaType != "text/plain" || document.Source.Data != "aGVsbG8=" {
		t.Fatalf("document = %#v", document)
	}
}

func TestDocumentTranslationRejectsProviderLocalReferences(t *testing.T) {
	tests := []struct {
		name      string
		translate func(llm.Request) (llm.Request, error)
		source    llm.MediaSource
	}{
		{
			name:      "anthropic file id to gemini",
			translate: AnthropicToGeminiRequest,
			source:    llm.MediaSource{Type: llm.MediaSourceFile, FileID: "file_1"},
		},
		{
			name:      "gemini file uri to anthropic",
			translate: GeminiToAnthropicRequest,
			source:    llm.MediaSource{Type: llm.MediaSourceURL, URL: "gemini://files/1", MediaType: "application/pdf"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.translate(llm.Request{
				Model: "m",
				Messages: []llm.Message{{
					Role:    llm.RoleUser,
					Content: []llm.ContentBlock{llm.DocumentBlock{Source: test.source}},
				}},
			})
			if !errors.Is(err, ErrUnsupported) {
				t.Fatalf("error = %v, want ErrUnsupported", err)
			}
		})
	}
}
