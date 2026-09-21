package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

func TestOpenAIChatToAnthropicStreamTranslatesSSE(t *testing.T) {
	anthropicSSE := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude","usage":{"input_tokens":4,"output_tokens":0}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(anthropicSSE)),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIChatToAnthropicStream(context.Background(), provider.Target{
		ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example",
	}, "claude-upstream", http.Header{}, []byte(`{"model":"portable","stream":true,"stream_options":{"include_usage":true},"max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("OpenAIChatToAnthropicStream() error = %v", err)
	}
	defer response.Body.Close()

	var sent struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode translated request: %v", err)
	}
	if sent.Model != "claude-upstream" || !sent.Stream {
		t.Fatalf("translated request = %#v", sent)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}
	text := string(body)
	for _, want := range []string{"chat.completion.chunk", `"content":"hello"`, `"prompt_tokens":4`, "data: [DONE]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("translated stream missing %q:\n%s", want, text)
		}
	}
}

func TestAnthropicMessagesToOpenAIStreamTranslatesSSE(t *testing.T) {
	openAISSE := strings.Join([]string{
		`data: {"id":"chat_1","object":"chat.completion.chunk","model":"gpt","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chat_1","object":"chat.completion.chunk","model":"gpt","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chat_1","object":"chat.completion.chunk","model":"gpt","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		`data: {"id":"chat_1","object":"chat.completion.chunk","model":"gpt","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(openAISSE)),
	}}
	runtime := New(client)
	response, err := runtime.AnthropicMessagesToOpenAIStream(context.Background(), provider.Target{
		ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example",
	}, "gpt-upstream", http.Header{}, []byte(`{"model":"portable","stream":true,"max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("AnthropicMessagesToOpenAIStream() error = %v", err)
	}
	defer response.Body.Close()

	var sent struct {
		Model         string `json:"model"`
		Stream        bool   `json:"stream"`
		StreamOptions struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode translated request: %v", err)
	}
	if sent.Model != "gpt-upstream" || !sent.Stream || !sent.StreamOptions.IncludeUsage {
		t.Fatalf("translated request = %#v", sent)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}
	text := string(body)
	for _, want := range []string{"event: message_start", `"text":"hello"`, `"input_tokens":4`, "event: message_stop"} {
		if !strings.Contains(text, want) {
			t.Fatalf("translated stream missing %q:\n%s", want, text)
		}
	}
}
