package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeChatRequestDisablesParallelToolCalls(t *testing.T) {
	request, err := DecodeChatRequest([]byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"parallel_tool_calls":false}`))
	if err != nil {
		t.Fatalf("DecodeChatRequest() error = %v", err)
	}
	if request.ToolChoice == nil || request.ToolChoice.Mode != llm.ToolChoiceAuto || !request.ToolChoice.DisableParallel {
		t.Fatalf("tool choice = %#v", request.ToolChoice)
	}
	if _, ok := request.Metadata["parallel_tool_calls"]; ok {
		t.Fatal("parallel_tool_calls leaked into metadata")
	}
}

func TestEncodeChatRequestDisablesParallelToolCalls(t *testing.T) {
	request := llm.Request{
		Model:      "m",
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		ToolChoice: &llm.ToolChoice{Mode: llm.ToolChoiceAuto, DisableParallel: true},
	}
	encoded, err := EncodeChatRequest(request)
	if err != nil {
		t.Fatalf("EncodeChatRequest() error = %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	var parallel bool
	if err := json.Unmarshal(wire["parallel_tool_calls"], &parallel); err != nil {
		t.Fatalf("decode parallel_tool_calls: %v", err)
	}
	if parallel {
		t.Fatal("parallel_tool_calls = true, want false")
	}
}
