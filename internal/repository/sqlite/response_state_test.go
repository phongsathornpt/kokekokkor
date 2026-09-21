package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
)

func TestResponseStateRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	record := responsestate.Record{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: []llm.ContentBlock{
				llm.TextBlock{Text: "question"},
				llm.DocumentBlock{
					Name: "note.txt",
					Source: llm.MediaSource{
						Type:      llm.MediaSourceBase64,
						MediaType: "text/plain",
						Data:      "aGVsbG8=",
					},
				},
			}},
			{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
				llm.TextBlock{Text: "answer"},
				llm.ToolCallBlock{ID: "call_1", Name: "lookup", Arguments: []byte(`{"id":1}`)},
			}},
		},
		Continuable: true,
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := store.SaveResponse(ctx, "resp_1", record); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}
	loaded, err := store.LoadResponse(ctx, "resp_1")
	if err != nil {
		t.Fatalf("LoadResponse() error = %v", err)
	}
	if !loaded.Continuable || len(loaded.Messages) != 2 {
		t.Fatalf("loaded = %#v", loaded)
	}
	text, ok := loaded.Messages[1].Content[0].(llm.TextBlock)
	if !ok || text.Text != "answer" {
		t.Fatalf("assistant text = %#v", loaded.Messages[1].Content)
	}
}

func TestResponseStateExpiry(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if err := store.SaveResponse(ctx, "expired", responsestate.Record{
		Continuable: true,
		ExpiresAt:   time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}
	if _, err := store.LoadResponse(ctx, "expired"); !errors.Is(err, responsestate.ErrNotFound) {
		t.Fatalf("LoadResponse() error = %v, want ErrNotFound", err)
	}
}
