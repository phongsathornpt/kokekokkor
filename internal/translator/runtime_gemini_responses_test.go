package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestOpenAIResponsesToGeminiTranslatesBufferedRequestAndResponse(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2,"thoughtsTokenCount":1},"modelVersion":"gemini-upstream","responseId":"gemini_resp_1"}`),
	}}
	runtime := New(client)
	response, err := runtime.OpenAIResponsesToGemini(context.Background(), provider.Target{
		ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://gemini.example",
	}, "gemini-upstream", http.Header{}, []byte(`{"model":"portable","instructions":"be concise","input":"hi","max_output_tokens":32}`))
	if err != nil {
		t.Fatalf("OpenAIResponsesToGemini() error = %v", err)
	}
	if client.request.Path != "/v1beta/models/gemini-upstream:generateContent" {
		t.Fatalf("upstream path = %q", client.request.Path)
	}

	var sent struct {
		SystemInstruction struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"systemInstruction"`
		Contents []struct {
			Role string `json:"role"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(client.request.Body, &sent); err != nil {
		t.Fatalf("decode Gemini request: %v", err)
	}
	if len(sent.SystemInstruction.Parts) != 1 || sent.SystemInstruction.Parts[0].Text != "be concise" {
		t.Fatalf("systemInstruction = %#v", sent.SystemInstruction)
	}
	if len(sent.Contents) != 1 || sent.Contents[0].Role != "user" {
		t.Fatalf("contents = %#v", sent.Contents)
	}

	var output struct {
		Object     string `json:"object"`
		Status     string `json:"status"`
		Model      string `json:"model"`
		OutputText string `json:"output_text"`
		Usage      struct {
			InputTokens   int64 `json:"input_tokens"`
			OutputTokens  int64 `json:"output_tokens"`
			OutputDetails struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(response.Body, &output); err != nil {
		t.Fatalf("decode Responses response: %v", err)
	}
	if output.Object != "response" || output.Status != "completed" || output.Model != "gemini-upstream" || output.OutputText != "hello" {
		t.Fatalf("translated response = %#v", output)
	}
	if output.Usage.InputTokens != 4 || output.Usage.OutputTokens != 2 || output.Usage.OutputDetails.ReasoningTokens != 1 {
		t.Fatalf("translated usage = %#v", output.Usage)
	}
}
