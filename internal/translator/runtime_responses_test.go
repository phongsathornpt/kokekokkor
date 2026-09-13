package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestOpenAIResponsesToAnthropicTranslatesBothDirections(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":"hello"},{"type":"tool_use","id":"call_1","name":"lookup","input":{"id":1}}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":6,"output_tokens":3,"cache_read_input_tokens":2,"cache_creation_input_tokens":1}}`),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIResponsesToAnthropic(context.Background(), provider.Target{
		ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example",
	}, "claude-upstream", http.Header{}, []byte(`{
		"model":"portable",
		"instructions":"be concise",
		"input":"hello",
		"max_output_tokens":32
	}`))
	if err != nil {
		t.Fatalf("OpenAIResponsesToAnthropic() error = %v", err)
	}
	if client.request.Path != "/v1/messages" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}

	var sent struct {
		Model  string `json:"model"`
		System any    `json:"system"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode Anthropic request: %v", err)
	}
	if sent.Model != "claude-upstream" || sent.System == nil {
		t.Fatalf("translated request = %#v", sent)
	}

	var output struct {
		Object     string `json:"object"`
		Status     string `json:"status"`
		OutputText string `json:"output_text"`
		Output     []struct {
			Type   string `json:"type"`
			CallID string `json:"call_id"`
		} `json:"output"`
		Usage struct {
			InputDetails struct {
				Cached     int64 `json:"cached_tokens"`
				CacheWrite int64 `json:"cache_write_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(response.Body, &output); err != nil {
		t.Fatalf("decode Responses output: %v", err)
	}
	if output.Object != "response" || output.Status != "completed" || output.OutputText != "hello" {
		t.Fatalf("translated response = %#v", output)
	}
	if len(output.Output) != 2 || output.Output[1].Type != "function_call" || output.Output[1].CallID != "call_1" {
		t.Fatalf("translated output items = %#v", output.Output)
	}
	if output.Usage.InputDetails.Cached != 2 || output.Usage.InputDetails.CacheWrite != 1 {
		t.Fatalf("translated usage = %#v", output.Usage)
	}
}
