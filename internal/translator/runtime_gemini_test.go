package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/upstream"
)

func TestOpenAIChatToGeminiTranslatesRequestAndResponse(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"responseId":"resp_1","modelVersion":"gemini-upstream","candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2}}`),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIChatToGemini(context.Background(), provider.Target{
		ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://gemini.example",
	}, "gemini-upstream", http.Header{}, []byte(`{"model":"portable","messages":[{"role":"system","content":"be concise"},{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("OpenAIChatToGemini() error = %v", err)
	}
	if client.request.Path != "/v1beta/models/gemini-upstream:generateContent" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}
	var sent struct {
		Contents []struct {
			Role string `json:"role"`
		} `json:"contents"`
		SystemInstruction *struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"systemInstruction"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode Gemini request: %v", err)
	}
	if len(sent.Contents) != 1 || sent.Contents[0].Role != "user" || sent.SystemInstruction == nil || len(sent.SystemInstruction.Parts) != 1 || sent.SystemInstruction.Parts[0].Text != "be concise" {
		t.Fatalf("translated request = %#v", sent)
	}
	var output struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(response.Body, &output); err != nil {
		t.Fatalf("decode OpenAI response: %v", err)
	}
	if output.Model != "gemini-upstream" || len(output.Choices) != 1 || output.Choices[0].Message.Content != "hello" {
		t.Fatalf("translated response = %#v", output)
	}
}

func TestGeminiGenerateContentToOpenAITranslatesRequestAndResponse(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"chatcmpl_1","object":"chat.completion","created":1,"model":"gpt-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`),
	}}
	runtime := New(client)
	response, err := runtime.GeminiGenerateContentToOpenAI(context.Background(), provider.Target{
		ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example",
	}, "gpt-upstream", http.Header{}, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":32}}`))
	if err != nil {
		t.Fatalf("GeminiGenerateContentToOpenAI() error = %v", err)
	}
	if client.request.Path != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}
	var sent struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode OpenAI request: %v", err)
	}
	if sent.Model != "gpt-upstream" || len(sent.Messages) != 1 || sent.Messages[0].Content != "hi" {
		t.Fatalf("translated request = %#v", sent)
	}
	var output struct {
		ModelVersion string `json:"modelVersion"`
		Candidates   []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(response.Body, &output); err != nil {
		t.Fatalf("decode Gemini response: %v", err)
	}
	if output.ModelVersion != "gpt-upstream" || len(output.Candidates) != 1 || len(output.Candidates[0].Content.Parts) != 1 || output.Candidates[0].Content.Parts[0].Text != "hello" {
		t.Fatalf("translated response = %#v", output)
	}
}

func TestOpenAIChatToAntigravityTranslatesRequestAndResponse(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"responseId":"resp_antigravity","modelVersion":"claude-opus-4-6-thinking","candidates":[{"content":{"role":"model","parts":[{"text":"Hello from Antigravity"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":8}}`),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIChatToGemini(context.Background(), provider.Target{
		ID: "antigravity", Protocol: provider.ProtocolGemini, BaseURL: "https://cloudcode-pa.googleapis.com",
	}, "claude-opus-4-6-thinking", http.Header{}, []byte(`{"model":"claude-opus-4-6-thinking","messages":[{"role":"user","content":"explain quantum gravity"}]}`))
	if err != nil {
		t.Fatalf("OpenAIChatToGemini() error = %v", err)
	}

	// Verify Antigravity path
	if client.request.Path != "/v1internal:generateContent" {
		t.Fatalf("upstream path = %q, want /v1internal:generateContent", client.request.Path)
	}

	// Verify Antigravity headers
	if client.request.Header.Get("User-Agent") != "antigravity/1.107.0 darwin/arm64" {
		t.Errorf("User-Agent = %q", client.request.Header.Get("User-Agent"))
	}
	if client.request.Header.Get("X-Client-Name") != "antigravity" {
		t.Errorf("X-Client-Name = %q", client.request.Header.Get("X-Client-Name"))
	}

	// Verify Antigravity request payload has model and ideRequestId
	var sent map[string]any
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode Antigravity request body: %v", err)
	}
	if sent["model"] != "claude-opus-4-6-thinking" {
		t.Errorf("model = %v, want claude-opus-4-6-thinking", sent["model"])
	}
	if _, ok := sent["ideRequestId"].(string); !ok {
		t.Errorf("ideRequestId missing or not string: %v", sent["ideRequestId"])
	}

	// Verify OpenAI response format
	var output struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(response.Body, &output); err != nil {
		t.Fatalf("decode OpenAI response: %v", err)
	}
	if output.Model != "claude-opus-4-6-thinking" || len(output.Choices) != 1 || output.Choices[0].Message.Content != "Hello from Antigravity" {
		t.Fatalf("output = %#v", output)
	}
}
