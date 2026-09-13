package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeResponsesRequestPortableItems(t *testing.T) {
	body := []byte(`{
		"model":"portable",
		"instructions":"be concise",
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"inspect"},
				{"type":"input_image","image_url":"https://example.com/image.png"}
			]},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"id\":1}"},
			{"type":"function_call_output","call_id":"call_1","output":"done"}
		],
		"tools":[{"type":"function","name":"lookup","description":"Lookup","parameters":{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"]}}],
		"tool_choice":"auto",
		"parallel_tool_calls":false,
		"max_output_tokens":64,
		"temperature":0.2,
		"stream":false
	}`)

	request, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest() error = %v", err)
	}
	if request.Model != "portable" || len(request.Messages) != 4 || len(request.Tools) != 1 {
		t.Fatalf("decoded request = %#v", request)
	}
	if request.Messages[0].Role != llm.RoleDeveloper {
		t.Fatalf("instruction role = %q", request.Messages[0].Role)
	}
	if request.ToolChoice == nil || request.ToolChoice.Mode != llm.ToolChoiceAuto || !request.ToolChoice.DisableParallel {
		t.Fatalf("tool choice = %#v", request.ToolChoice)
	}
	if request.MaxOutputTokens == nil || *request.MaxOutputTokens != 64 {
		t.Fatalf("max output tokens = %#v", request.MaxOutputTokens)
	}
	if raw := request.Metadata["stream"]; string(raw) != "false" {
		t.Fatalf("stream metadata = %s", raw)
	}
}

func TestDecodeResponsesRequestRejectsBuiltInTool(t *testing.T) {
	_, err := DecodeResponsesRequest([]byte(`{
		"model":"portable",
		"input":"hello",
		"tools":[{"type":"web_search"}],
		"max_output_tokens":32
	}`))
	if err == nil {
		t.Fatal("DecodeResponsesRequest() error = nil, want unsupported tool error")
	}
}

func TestEncodeResponsesResponseTextAndToolCall(t *testing.T) {
	encoded, err := EncodeResponsesResponse(llm.Response{
		ID:    "msg_upstream",
		Model: "claude-upstream",
		Content: []llm.ContentBlock{
			llm.TextBlock{Text: "hello"},
			llm.ToolCallBlock{ID: "call_1", Name: "lookup", Arguments: json.RawMessage(`{"id":1}`)},
		},
		StopReason: llm.StopReasonToolUse,
		Usage: llm.Usage{
			InputTokens:      10,
			OutputTokens:     4,
			CacheReadTokens:  3,
			CacheWriteTokens: 2,
		},
	})
	if err != nil {
		t.Fatalf("EncodeResponsesResponse() error = %v", err)
	}

	var response struct {
		Object     string `json:"object"`
		Status     string `json:"status"`
		OutputText string `json:"output_text"`
		Output     []struct {
			Type      string `json:"type"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"output"`
		Usage struct {
			InputDetails struct {
				Cached     int64 `json:"cached_tokens"`
				CacheWrite int64 `json:"cache_write_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode encoded response: %v", err)
	}
	if response.Object != "response" || response.Status != "completed" || response.OutputText != "hello" {
		t.Fatalf("response = %#v", response)
	}
	if len(response.Output) != 2 || response.Output[1].Type != "function_call" || response.Output[1].CallID != "call_1" {
		t.Fatalf("output = %#v", response.Output)
	}
	if response.Usage.InputDetails.Cached != 3 || response.Usage.InputDetails.CacheWrite != 2 {
		t.Fatalf("usage = %#v", response.Usage)
	}
}
