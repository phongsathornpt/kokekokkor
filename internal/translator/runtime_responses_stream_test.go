package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestOpenAIResponsesToAnthropicStreamTranslatesSSE(t *testing.T) {
	anthropicSSE := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-upstream","usage":{"input_tokens":4,"output_tokens":0}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2,"cache_read_input_tokens":1,"cache_creation_input_tokens":3}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(anthropicSSE)),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIResponsesToAnthropicStream(
		context.Background(),
		provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example"},
		"claude-upstream",
		http.Header{},
		[]byte(`{"model":"portable","stream":true,"stream_options":{"include_obfuscation":false},"max_output_tokens":32,"input":"hi"}`),
	)
	if err != nil {
		t.Fatalf("OpenAIResponsesToAnthropicStream() error = %v", err)
	}
	defer response.Body.Close()

	if client.request.Path != "/v1/messages" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode translated request: %v", err)
	}
	var model string
	if err := json.Unmarshal(sent["model"], &model); err != nil || model != "claude-upstream" {
		t.Fatalf("upstream model = %q, err=%v", model, err)
	}
	var stream bool
	if err := json.Unmarshal(sent["stream"], &stream); err != nil || !stream {
		t.Fatalf("upstream stream = %v, err=%v", stream, err)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}
	text := string(body)
	for _, event := range []string{
		"response.created",
		"response.output_text.delta",
		"response.output_text.done",
		"response.completed",
	} {
		if !strings.Contains(text, "event: "+event) {
			t.Fatalf("translated stream missing %s: %s", event, text)
		}
	}
	if !strings.Contains(text, `"output_text":"hello"`) {
		t.Fatalf("translated terminal response has no output text: %s", text)
	}
	if !strings.Contains(text, `"cache_write_tokens":3`) {
		t.Fatalf("translated terminal response has no cache-write usage: %s", text)
	}
	if strings.Contains(text, "[DONE]") {
		t.Fatalf("Responses stream contains [DONE]: %s", text)
	}
}
