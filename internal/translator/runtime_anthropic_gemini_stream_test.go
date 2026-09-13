package translator

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestAnthropicMessagesToGeminiStream(t *testing.T) {
	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"hello\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":2},\"modelVersion\":\"gemini-upstream\",\"responseId\":\"gemini_resp_1\"}\n\n",
		)),
	}}
	runtime := New(client)
	response, err := runtime.AnthropicMessagesToGeminiStream(context.Background(), provider.Target{
		ID: "gemini", Protocol: provider.ProtocolGemini,
	}, "gemini-upstream", http.Header{}, []byte(`{"model":"portable","max_tokens":32,"stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("AnthropicMessagesToGeminiStream() error = %v", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}
	if client.request.Path != "/v1beta/models/gemini-upstream:streamGenerateContent" || client.request.RawQuery != "alt=sse" {
		t.Fatalf("upstream request path=%q query=%q", client.request.Path, client.request.RawQuery)
	}
	text := string(data)
	for _, want := range []string{"message_start", "content_block_delta", "hello", "message_stop"} {
		if !strings.Contains(text, want) {
			t.Fatalf("translated Anthropic stream missing %q: %q", want, text)
		}
	}
}

func TestGeminiStreamGenerateContentToAnthropic(t *testing.T) {
	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-upstream\",\"usage\":{\"input_tokens\":4,\"output_tokens\":0}}}\n\n" +
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
				"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":2}}\n\n" +
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		)),
	}}
	runtime := New(client)
	response, err := runtime.GeminiStreamGenerateContentToAnthropic(context.Background(), provider.Target{
		ID: "anthropic", Protocol: provider.ProtocolAnthropic,
	}, "claude-upstream", http.Header{}, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":32}}`))
	if err != nil {
		t.Fatalf("GeminiStreamGenerateContentToAnthropic() error = %v", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}
	if client.request.Path != "/v1/messages" {
		t.Fatalf("upstream request path=%q", client.request.Path)
	}
	if !strings.Contains(string(client.request.Body), `"stream":true`) {
		t.Fatalf("translated Anthropic request = %s", client.request.Body)
	}
	text := string(data)
	if !strings.Contains(text, `"text":"hello"`) || !strings.Contains(text, `"finishReason":"STOP"`) {
		t.Fatalf("translated Gemini stream = %q", text)
	}
	if strings.Contains(text, "[DONE]") {
		t.Fatalf("Gemini stream must not emit [DONE]: %q", text)
	}
}
