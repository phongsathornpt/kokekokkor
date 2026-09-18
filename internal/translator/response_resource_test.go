package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestStoredResponseResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	runtime := &Runtime{responseState: newMemoryResponseStateStore()}
	plan := responseStatePlan{
		state: &llm.ResponseState{Store: true},
		history: []llm.Message{{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}},
		}},
	}
	if err := runtime.persistResponsesState(ctx, plan, llm.Response{
		ID:         "provider_1",
		Model:      "translated-model",
		Content:    []llm.ContentBlock{llm.TextBlock{Text: "answer"}},
		StopReason: llm.StopReasonEndTurn,
	}); err != nil {
		t.Fatalf("persistResponsesState() error = %v", err)
	}

	status, payload, handled, err := runtime.HandleStoredResponse(ctx, http.MethodGet, "/v1/responses/resp_provider_1")
	if err != nil || !handled || status != http.StatusOK {
		t.Fatalf("retrieve = status %d handled %v err %v", status, handled, err)
	}
	var response struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode stored response: %v", err)
	}
	if response.ID != "resp_provider_1" || response.Object != "response" {
		t.Fatalf("response = %#v", response)
	}

	status, _, handled, err = runtime.HandleStoredResponse(ctx, http.MethodDelete, "/v1/responses/resp_provider_1")
	if err != nil || !handled || status != http.StatusOK {
		t.Fatalf("delete = status %d handled %v err %v", status, handled, err)
	}
	_, _, handled, err = runtime.HandleStoredResponse(ctx, http.MethodGet, "/v1/responses/resp_provider_1")
	if err != nil || handled {
		t.Fatalf("retrieve deleted = handled %v err %v", handled, err)
	}
}
