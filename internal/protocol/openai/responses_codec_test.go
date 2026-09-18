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

func TestDecodeResponsesRequestInlineFile(t *testing.T) {
	request, err := DecodeResponsesRequest([]byte(`{
		"model":"portable",
		"input":[{"role":"user","content":[
			{"type":"input_file","filename":"note.txt","file_data":"data:text/plain;base64,aGVsbG8="}
		]}]
	}`))
	if err != nil {
		t.Fatalf("DecodeResponsesRequest() error = %v", err)
	}
	if len(request.Messages) != 1 || len(request.Messages[0].Content) != 1 {
		t.Fatalf("messages = %#v", request.Messages)
	}
	doc, ok := request.Messages[0].Content[0].(llm.DocumentBlock)
	if !ok {
		t.Fatalf("content = %#v", request.Messages[0].Content[0])
	}
	if doc.Name != "note.txt" || doc.Source.Type != llm.MediaSourceBase64 || doc.Source.MediaType != "text/plain" || doc.Source.Data != "aGVsbG8=" {
		t.Fatalf("document = %#v", doc)
	}
}

func TestDecodeResponsesRequestRejectsInvalidInlineFile(t *testing.T) {
	_, err := DecodeResponsesRequest([]byte(`{
		"model":"portable",
		"input":[{"role":"user","content":[
			{"type":"input_file","file_data":"data:text/plain;base64,%%%"}
		]}]
	}`))
	if err == nil {
		t.Fatal("DecodeResponsesRequest() error = nil, want invalid base64 error")
	}
}

func TestDecodeResponsesRequestWebSearch(t *testing.T) {
	request, err := DecodeResponsesRequest([]byte(`{
		"model":"portable",
		"input":"hello",
		"tools":[{"type":"web_search"}],
		"max_output_tokens":32
	}`))
	if err != nil {
		t.Fatalf("DecodeResponsesRequest() error = %v", err)
	}
	if len(request.Tools) != 1 || request.Tools[0].Kind != llm.ToolKindWebSearch {
		t.Fatalf("tools = %#v", request.Tools)
	}
}

func TestDecodeResponsesRequestRejectsWebSearchOptions(t *testing.T) {
	_, err := DecodeResponsesRequest([]byte(`{
		"model":"portable",
		"input":"hello",
		"tools":[{"type":"web_search","search_context_size":"low"}],
		"max_output_tokens":32
	}`))
	if err == nil {
		t.Fatal("DecodeResponsesRequest() error = nil, want unsupported options error")
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

func TestEncodeResponsesResponseURLCitation(t *testing.T) {
	encoded, err := EncodeResponsesResponse(llm.Response{
		ID:    "resp_cited",
		Model: "gemini-upstream",
		Content: []llm.ContentBlock{llm.TextBlock{
			Text: "grounded answer",
			Citations: []llm.URLCitation{{
				StartIndex: 0,
				EndIndex:   8,
				URL:        "https://example.com/source",
				Title:      "Source",
			}},
		}},
		StopReason: llm.StopReasonEndTurn,
	})
	if err != nil {
		t.Fatalf("EncodeResponsesResponse() error = %v", err)
	}
	var response struct {
		Output []struct {
			Content []struct {
				Annotations []struct {
					Type       string `json:"type"`
					StartIndex int    `json:"start_index"`
					EndIndex   int    `json:"end_index"`
					URL        string `json:"url"`
					Title      string `json:"title"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode encoded response: %v", err)
	}
	annotation := response.Output[0].Content[0].Annotations[0]
	if annotation.Type != "url_citation" || annotation.StartIndex != 0 || annotation.EndIndex != 8 || annotation.URL != "https://example.com/source" || annotation.Title != "Source" {
		t.Fatalf("annotation = %#v", annotation)
	}
}

func TestEncodeResponsesResponseWebSearchCall(t *testing.T) {
	encoded, err := EncodeResponsesResponse(llm.Response{
		ID:    "resp_search",
		Model: "gemini-upstream",
		Content: []llm.ContentBlock{
			llm.WebSearchCallBlock{Queries: []string{"query one", "query two"}},
			llm.TextBlock{Text: "answer"},
		},
		StopReason: llm.StopReasonEndTurn,
	})
	if err != nil {
		t.Fatalf("EncodeResponsesResponse() error = %v", err)
	}
	var response struct {
		Output []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Action *struct {
				Type    string   `json:"type"`
				Queries []string `json:"queries"`
			} `json:"action"`
		} `json:"output"`
	}
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode encoded response: %v", err)
	}
	if len(response.Output) != 2 || response.Output[0].Type != "web_search_call" || response.Output[0].Status != "completed" || response.Output[0].Action == nil {
		t.Fatalf("output = %#v", response.Output)
	}
	if response.Output[0].Action.Type != "search" || len(response.Output[0].Action.Queries) != 2 || response.Output[0].Action.Queries[0] != "query one" {
		t.Fatalf("action = %#v", response.Output[0].Action)
	}
}

func TestEncodeResponsesResponseRefusal(t *testing.T) {
	encoded, err := EncodeResponsesResponse(llm.Response{
		ID:         "msg_refusal",
		Model:      "claude-upstream",
		Content:    []llm.ContentBlock{llm.TextBlock{Text: "I can't help with that."}},
		StopReason: llm.StopReasonContentBlock,
		Usage:      llm.Usage{InputTokens: 8, OutputTokens: 6},
	})
	if err != nil {
		t.Fatalf("EncodeResponsesResponse() error = %v", err)
	}

	var response struct {
		Status     string `json:"status"`
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode encoded response: %v", err)
	}
	if response.Status != "completed" {
		t.Fatalf("status = %q", response.Status)
	}
	if response.OutputText != "" {
		t.Fatalf("output_text = %q, want empty for refusal", response.OutputText)
	}
	if len(response.Output) != 1 || response.Output[0].Type != "message" || len(response.Output[0].Content) != 1 {
		t.Fatalf("output = %#v", response.Output)
	}
	part := response.Output[0].Content[0]
	if part.Type != "refusal" || part.Refusal != "I can't help with that." || part.Text != "" {
		t.Fatalf("refusal part = %#v", part)
	}
}
