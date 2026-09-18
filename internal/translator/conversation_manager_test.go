package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestConversationLifecycleAndResponseState(t *testing.T) {
	ctx := context.Background()
	runtime := &Runtime{responseState: newMemoryResponseStateStore()}

	status, body, handled, err := runtime.HandleConversation(ctx, http.MethodPost, "/v1/conversations", []byte(`{
		"metadata":{"topic":"demo"},
		"items":[{"type":"message","role":"user","content":"initial"}]
	}`))
	if err != nil {
		t.Fatalf("create conversation error = %v", err)
	}
	if !handled || status != http.StatusOK {
		t.Fatalf("create = handled %v status %d", handled, status)
	}
	var created struct {
		ID       string            `json:"id"`
		Object   string            `json:"object"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Object != "conversation" || created.Metadata["topic"] != "demo" {
		t.Fatalf("created = %#v", created)
	}

	request := llm.Request{
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "follow up"}},
		}},
		ResponseState: &llm.ResponseState{ConversationID: created.ID},
	}
	resolved, plan, err := runtime.resolveResponsesState(ctx, request)
	if err != nil {
		t.Fatalf("resolveResponsesState() error = %v", err)
	}
	if len(resolved.Messages) != 2 {
		t.Fatalf("resolved messages = %#v", resolved.Messages)
	}
	if err := runtime.persistResponsesState(ctx, plan, llm.Response{
		ID:      "resp_conv",
		Content: []llm.ContentBlock{llm.TextBlock{Text: "answer"}},
	}); err != nil {
		t.Fatalf("persistResponsesState() error = %v", err)
	}

	conversation, err := runtime.responseState.LoadConversation(ctx, created.ID)
	if err != nil {
		t.Fatalf("LoadConversation() error = %v", err)
	}
	if len(conversation.Messages) != 3 {
		t.Fatalf("conversation messages = %#v", conversation.Messages)
	}
	got := []string{
		conversation.Messages[0].Content[0].(llm.TextBlock).Text,
		conversation.Messages[1].Content[0].(llm.TextBlock).Text,
		conversation.Messages[2].Content[0].(llm.TextBlock).Text,
	}
	want := []string{"initial", "follow up", "answer"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("message %d = %q, want %q", i, got[i], want[i])
		}
	}

	status, body, handled, err = runtime.HandleConversation(ctx, http.MethodGet, "/v1/conversations/"+created.ID, nil)
	if err != nil || !handled || status != http.StatusOK {
		t.Fatalf("retrieve = status %d handled %v err %v", status, handled, err)
	}

	status, _, handled, err = runtime.HandleConversation(ctx, http.MethodDelete, "/v1/conversations/"+created.ID, nil)
	if err != nil || !handled || status != http.StatusOK {
		t.Fatalf("delete = status %d handled %v err %v", status, handled, err)
	}
	status, _, handled, err = runtime.HandleConversation(ctx, http.MethodGet, "/v1/conversations/"+created.ID, nil)
	if err != nil || !handled || status != http.StatusNotFound {
		t.Fatalf("retrieve deleted = status %d handled %v err %v", status, handled, err)
	}
}
