package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type fakeClient struct {
	request  upstream.Request
	response upstream.Response
}

func (f *fakeClient) Do(_ context.Context, _ provider.Target, request upstream.Request) (upstream.Response, error) {
	f.request = request
	return f.response, nil
}

func TestOpenAIChatToAnthropicTranslatesBothDirections(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":4,"output_tokens":2}}`),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIChatToAnthropic(context.Background(), provider.Target{
		ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example",
	}, "claude-upstream", http.Header{}, []byte(`{"model":"portable","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("OpenAIChatToAnthropic() error = %v", err)
	}
	if client.request.Path != "/v1/messages" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}
	var sent struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil || sent.Model != "claude-upstream" {
		t.Fatalf("translated request model = %q, err=%v", sent.Model, err)
	}
	var output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(response.Body, &output); err != nil {
		t.Fatalf("decode translated response: %v", err)
	}
	if len(output.Choices) != 1 || output.Choices[0].Message.Content != "hello" {
		t.Fatalf("translated response = %#v", output)
	}
}
