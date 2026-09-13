package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/application/upstream"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestAnthropicMessagesToGemini(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2},"modelVersion":"gemini-upstream","responseId":"resp_1"}`),
	}}
	runtime := New(client)
	response, err := runtime.AnthropicMessagesToGemini(context.Background(), provider.Target{ID: "gemini", Protocol: provider.ProtocolGemini}, "gemini-upstream", http.Header{}, []byte(`{"model":"portable","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil { t.Fatalf("AnthropicMessagesToGemini() error = %v", err) }
	if client.request.Path != "/v1beta/models/gemini-upstream:generateContent" { t.Fatalf("path = %q", client.request.Path) }
	var output struct { Content []struct { Text string `json:"text"` } `json:"content"` }
	if err := json.Unmarshal(response.Body, &output); err != nil { t.Fatalf("decode response: %v", err) }
	if len(output.Content) != 1 || output.Content[0].Text != "hello" { t.Fatalf("response = %s", response.Body) }
}

func TestGeminiGenerateContentToAnthropic(t *testing.T) {
	client := &fakeClient{response: upstream.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-upstream","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":4,"output_tokens":2}}`),
	}}
	runtime := New(client)
	response, err := runtime.GeminiGenerateContentToAnthropic(context.Background(), provider.Target{ID: "anthropic", Protocol: provider.ProtocolAnthropic}, "claude-upstream", http.Header{}, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
	if err != nil { t.Fatalf("GeminiGenerateContentToAnthropic() error = %v", err) }
	if client.request.Path != "/v1/messages" { t.Fatalf("path = %q", client.request.Path) }
	var output struct { Candidates []struct { Content struct { Parts []struct { Text *string `json:"text"` } `json:"parts"` } `json:"content"` } `json:"candidates"` }
	if err := json.Unmarshal(response.Body, &output); err != nil { t.Fatalf("decode response: %v", err) }
	if len(output.Candidates) != 1 || len(output.Candidates[0].Content.Parts) != 1 || output.Candidates[0].Content.Parts[0].Text == nil || *output.Candidates[0].Content.Parts[0].Text != "hello" { t.Fatalf("response = %s", response.Body) }
}
