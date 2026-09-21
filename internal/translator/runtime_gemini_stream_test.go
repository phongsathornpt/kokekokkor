package translator

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
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

func TestGeminiStreamGenerateContentToOpenAITranslatesNativeSSE(t *testing.T) {
	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-upstream","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
			``,
			`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-upstream","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
			``,
			`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-upstream","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			``,
			`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-upstream","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))),
	}}
	runtime := New(client)
	response, err := runtime.GeminiStreamGenerateContentToOpenAI(context.Background(), provider.Target{
		ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example",
	}, "gpt-upstream", http.Header{}, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	if err != nil {
		t.Fatalf("GeminiStreamGenerateContentToOpenAI() error = %v", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated Gemini stream: %v", err)
	}
	if client.request.Path != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}
	text := string(data)
	if !strings.Contains(text, `"text":"hello"`) || !strings.Contains(text, `"finishReason":"STOP"`) {
		t.Fatalf("translated Gemini stream = %q", text)
	}
	if strings.Contains(text, "[DONE]") {
		t.Fatalf("native Gemini stream must not emit [DONE]: %q", text)
	}
	if got := response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestOpenAIChatToAntigravityStreamTranslatesSSE(t *testing.T) {
	client := &fakeClient{streamResponse: upstream.StreamResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Streaming from Antigravity\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":2},\"modelVersion\":\"claude-opus-4-6-thinking\",\"responseId\":\"agy_resp_1\"}\n\n",
		)),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIChatToGeminiStream(context.Background(), provider.Target{
		ID: "antigravity", Protocol: provider.ProtocolGemini, BaseURL: "https://cloudcode-pa.googleapis.com",
	}, "claude-opus-4-6-thinking", http.Header{}, []byte(`{"model":"claude-opus-4-6-thinking","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("OpenAIChatToGeminiStream() error = %v", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}

	// Verify Antigravity stream path and query
	if client.request.Path != "/v1internal:streamGenerateContent" || client.request.RawQuery != "alt=sse" {
		t.Fatalf("upstream path=%q query=%q", client.request.Path, client.request.RawQuery)
	}

	// Verify Antigravity headers
	if client.request.Header.Get("User-Agent") != "antigravity/1.107.0 darwin/arm64" {
		t.Errorf("User-Agent = %q", client.request.Header.Get("User-Agent"))
	}
	if client.request.Header.Get("X-Client-Name") != "antigravity" {
		t.Errorf("X-Client-Name = %q", client.request.Header.Get("X-Client-Name"))
	}

	text := string(data)
	if !strings.Contains(text, "chat.completion.chunk") || !strings.Contains(text, "Streaming from Antigravity") || !strings.Contains(text, "[DONE]") {
		t.Fatalf("translated stream = %q", text)
	}
}
