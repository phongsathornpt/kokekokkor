package translator

import (
	"context"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
)

func TestResponsesStateContinuationDoesNotInheritInstructions(t *testing.T) {
	ctx := context.Background()
	runtime := &Runtime{responseState: newMemoryResponseStateStore()}

	first := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleDeveloper, Content: []llm.ContentBlock{llm.TextBlock{Text: "first instructions"}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "first question"}}},
		},
		ResponseState: &llm.ResponseState{Store: true, InstructionMessages: 1},
	}
	resolved, plan, err := runtime.resolveResponsesState(ctx, first)
	if err != nil {
		t.Fatalf("resolveResponsesState(first) error = %v", err)
	}
	if len(resolved.Messages) != 2 {
		t.Fatalf("resolved first messages = %#v", resolved.Messages)
	}
	if err := runtime.persistResponsesState(ctx, plan, llm.Response{
		ID:      "resp_1",
		Content: []llm.ContentBlock{llm.TextBlock{Text: "first answer"}},
	}); err != nil {
		t.Fatalf("persistResponsesState() error = %v", err)
	}

	second := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleDeveloper, Content: []llm.ContentBlock{llm.TextBlock{Text: "second instructions"}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "follow up"}}},
		},
		ResponseState: &llm.ResponseState{
			PreviousResponseID: "resp_1",
			InstructionMessages: 1,
		},
	}
	resolved, _, err = runtime.resolveResponsesState(ctx, second)
	if err != nil {
		t.Fatalf("resolveResponsesState(second) error = %v", err)
	}
	if len(resolved.Messages) != 4 {
		t.Fatalf("resolved continuation messages = %#v", resolved.Messages)
	}
	want := []string{"second instructions", "first question", "first answer", "follow up"}
	for i, expected := range want {
		text, ok := resolved.Messages[i].Content[0].(llm.TextBlock)
		if !ok || text.Text != expected {
			t.Fatalf("message %d = %#v, want %q", i, resolved.Messages[i], expected)
		}
	}
}

func TestResponsesStateRejectsNonportableContinuation(t *testing.T) {
	ctx := context.Background()
	store := newMemoryResponseStateStore()
	if err := store.SaveResponse(ctx, "resp_search", responsestate.Record{
		Continuable: false,
	}); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}
	runtime := &Runtime{responseState: store}
	_, _, err := runtime.resolveResponsesState(ctx, llm.Request{
		ResponseState: &llm.ResponseState{PreviousResponseID: "resp_search"},
	})
	if err == nil {
		t.Fatal("resolveResponsesState() error = nil, want nonportable continuation error")
	}
}
