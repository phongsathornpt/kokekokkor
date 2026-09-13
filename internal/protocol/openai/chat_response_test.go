package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeChatResponseMapsUsageAndToolCalls(t *testing.T) {
	data := []byte(`{
		"id":"chatcmpl_1",
		"object":"chat.completion",
		"created":1,
		"model":"upstream-model",
		"choices":[{
			"index":0,
			"message":{"role":"assistant","content":"checking","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]},
			"finish_reason":"tool_calls"
		}],
		"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens_details":{"reasoning_tokens":2}}
	}`)

	response, err := DecodeChatResponse(data)
	if err != nil {
		t.Fatalf("DecodeChatResponse() error = %v", err)
	}
	if response.ID != "chatcmpl_1" || response.Model != "upstream-model" || response.StopReason != llm.StopReasonToolUse {
		t.Fatalf("response = %#v", response)
	}
	if response.Usage.InputTokens != 11 || response.Usage.OutputTokens != 7 || response.Usage.CacheReadTokens != 3 || response.Usage.ReasoningTokens != 2 {
		t.Fatalf("usage = %#v", response.Usage)
	}
	if len(response.Content) != 2 {
		t.Fatalf("content = %#v", response.Content)
	}
	call, ok := response.Content[1].(llm.ToolCallBlock)
	if !ok || call.ID != "call_1" || call.Name != "lookup" {
		t.Fatalf("tool call = %#v", response.Content[1])
	}
}

func TestEncodeChatResponseProducesOneChoice(t *testing.T) {
	encoded, err := EncodeChatResponse(llm.Response{
		ID:    "msg_1",
		Model: "client-model",
		Content: []llm.ContentBlock{
			llm.TextBlock{Text: "hello"},
		},
		StopReason: llm.StopReasonEndTurn,
		Usage:      llm.Usage{InputTokens: 4, OutputTokens: 2},
	})
	if err != nil {
		t.Fatalf("EncodeChatResponse() error = %v", err)
	}

	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	var choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(wire["choices"], &choices); err != nil {
		t.Fatalf("decode choices: %v", err)
	}
	if len(choices) != 1 || choices[0].FinishReason != "stop" || choices[0].Message.Role != "assistant" || choices[0].Message.Content != "hello" {
		t.Fatalf("choices = %#v", choices)
	}
}

func TestDecodeChatResponseRejectsMultipleChoices(t *testing.T) {
	_, err := DecodeChatResponse([]byte(`{"model":"m","choices":[{"message":{"role":"assistant","content":"a"}},{"message":{"role":"assistant","content":"b"}}]}`))
	if err == nil {
		t.Fatal("DecodeChatResponse() error = nil, want multiple-choice error")
	}
}
