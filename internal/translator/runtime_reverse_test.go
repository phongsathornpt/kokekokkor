package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestAnthropicMessagesToOpenAITranslatesBothDirections(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"chatcmpl_1","object":"chat.completion","created":1,"model":"gpt","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`),
	}}
	runtime := New(client)
	response, err := runtime.AnthropicMessagesToOpenAI(context.Background(), provider.Target{
		ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example",
	}, "gpt-upstream", http.Header{}, []byte(`{"model":"portable","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("AnthropicMessagesToOpenAI() error = %v", err)
	}
	if client.request.Path != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}
	var sent struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil || sent.Model != "gpt-upstream" {
		t.Fatalf("translated request model = %q, err=%v", sent.Model, err)
	}
	var output struct {
		Type    string `json:"type"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response.Body, &output); err != nil {
		t.Fatalf("decode translated response: %v", err)
	}
	if output.Type != "message" || len(output.Content) != 1 || output.Content[0].Text != "hello" {
		t.Fatalf("translated response = %#v", output)
	}
}
