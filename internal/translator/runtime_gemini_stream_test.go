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

func TestOpenAIChatToGeminiStreamTranslatesSSE(t *testing.T) {
	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"hello\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":2},\"modelVersion\":\"gemini-upstream\",\"responseId\":\"gemini_resp_1\"}\n\n",
		)),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIChatToGeminiStream(context.Background(), provider.Target{
		ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://gemini.example",
	}, "gemini-upstream", http.Header{}, []byte(`{"model":"portable","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("OpenAIChatToGeminiStream() error = %v", err)
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
	if !strings.Contains(text, "chat.completion.chunk") || !strings.Contains(text, "hello") || !strings.Contains(text, "[DONE]") {
		t.Fatalf("translated stream = %q", text)
	}
	if got := response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestOpenAIResponsesToGeminiStreamTranslatesLifecycle(t *testing.T) {
	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"hello\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":2},\"modelVersion\":\"gemini-upstream\",\"responseId\":\"gemini_resp_2\"}\n\n",
		)),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIResponsesToGeminiStream(context.Background(), provider.Target{
		ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://gemini.example",
	}, "gemini-upstream", http.Header{}, []byte(`{"model":"portable","stream":true,"stream_options":{"include_obfuscation":false},"input":"hi"}`))
	if err != nil {
		t.Fatalf("OpenAIResponsesToGeminiStream() error = %v", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}
	text := string(data)
	for _, want := range []string{"response.created", "response.output_text.delta", "response.completed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("translated Responses stream missing %q: %q", want, text)
		}
	}
	if strings.Contains(text, "[DONE]") {
		t.Fatalf("Responses stream must not emit [DONE]: %q", text)
	}
}
