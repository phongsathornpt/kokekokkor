package llm

import (
	"errors"
	"testing"
)

func TestRequestValidateAcceptsPortableBlocksAndTools(t *testing.T) {
	request := Request{
		Model: "portable-model",
		Messages: []Message{
			{Role: RoleSystem, Content: []ContentBlock{TextBlock{Text: "be precise"}}},
			{Role: RoleUser, Content: []ContentBlock{
				TextBlock{Text: "inspect this"},
				ImageBlock{Source: MediaSource{Type: MediaSourceURL, URL: "https://example.com/image.png"}},
			}},
			{Role: RoleAssistant, Content: []ContentBlock{
				ToolCallBlock{ID: "call_1", Name: "lookup", Arguments: []byte(`{"q":"x"}`)},
			}},
			{Role: RoleUser, Content: []ContentBlock{
				ToolResultBlock{ToolCallID: "call_1", Content: []ContentBlock{TextBlock{Text: "done"}}},
			}},
		},
		Tools: []Tool{{Name: "lookup", InputSchema: []byte(`{"type":"object"}`)}},
		ToolChoice: &ToolChoice{
			Mode: ToolChoiceAuto,
		},
	}

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRequestValidateRejectsDuplicateTools(t *testing.T) {
	request := Request{Tools: []Tool{
		{Name: "lookup", InputSchema: []byte(`{"type":"object"}`)},
		{Name: "lookup", InputSchema: []byte(`{"type":"object"}`)},
	}}

	if err := request.Validate(); !errors.Is(err, ErrDuplicateToolName) {
		t.Fatalf("Validate() error = %v, want ErrDuplicateToolName", err)
	}
}

func TestRequestValidateRejectsInvalidToolCallArguments(t *testing.T) {
	request := Request{Messages: []Message{{
		Role: RoleAssistant,
		Content: []ContentBlock{
			ToolCallBlock{ID: "call_1", Name: "lookup", Arguments: []byte(`{"broken"`)},
		},
	}}}

	if err := request.Validate(); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("Validate() error = %v, want ErrInvalidContent", err)
	}
}

func TestRequestValidateRejectsInvalidMediaSource(t *testing.T) {
	request := Request{Messages: []Message{{
		Role: RoleUser,
		Content: []ContentBlock{
			ImageBlock{Source: MediaSource{Type: MediaSourceBase64, MediaType: "image/png"}},
		},
	}}}

	if err := request.Validate(); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("Validate() error = %v, want ErrInvalidContent", err)
	}
}
