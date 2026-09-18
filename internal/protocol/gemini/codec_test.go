package gemini

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestGenerateContentRequestRoundTripPortableSemantics(t *testing.T) {
	maxTokens := 128
	temperature := 0.4
	topP := 0.8
	request := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: []llm.ContentBlock{llm.TextBlock{Text: "be concise"}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "weather?"}}},
			{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.ToolCallBlock{ID: "call_1", Name: "weather", Arguments: json.RawMessage(`{"city":"Bangkok"}`)}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.ToolResultBlock{ToolCallID: "call_1", Content: []llm.ContentBlock{llm.TextBlock{Text: `{"temp":31}`}}}}},
		},
		Tools: []llm.Tool{{
			Name:        "weather",
			Description: "look up weather",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
		}},
		ToolChoice:      &llm.ToolChoice{Mode: llm.ToolChoiceNamed, Name: "weather"},
		MaxOutputTokens: &maxTokens,
		Temperature:     &temperature,
		TopP:            &topP,
		Stop:            []string{"END"},
	}

	encoded, err := EncodeGenerateContentRequest(request)
	if err != nil {
		t.Fatalf("EncodeGenerateContentRequest() error = %v", err)
	}
	decoded, err := DecodeGenerateContentRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeGenerateContentRequest() error = %v", err)
	}
	if len(decoded.Messages) != 4 {
		t.Fatalf("messages = %#v", decoded.Messages)
	}
	if got := decoded.Messages[0].Content[0].(llm.TextBlock).Text; got != "be concise" {
		t.Fatalf("system text = %q", got)
	}
	call, ok := decoded.Messages[2].Content[0].(llm.ToolCallBlock)
	if !ok || call.ID != "call_1" || call.Name != "weather" || string(call.Arguments) != `{"city":"Bangkok"}` {
		t.Fatalf("tool call = %#v", decoded.Messages[2].Content)
	}
	result, ok := decoded.Messages[3].Content[0].(llm.ToolResultBlock)
	if !ok || result.ToolCallID != "call_1" {
		t.Fatalf("tool result = %#v", decoded.Messages[3].Content)
	}
	if len(decoded.Tools) != 1 || decoded.Tools[0].Name != "weather" {
		t.Fatalf("tools = %#v", decoded.Tools)
	}
	if decoded.ToolChoice == nil || decoded.ToolChoice.Mode != llm.ToolChoiceNamed || decoded.ToolChoice.Name != "weather" {
		t.Fatalf("tool choice = %#v", decoded.ToolChoice)
	}
	if decoded.MaxOutputTokens == nil || *decoded.MaxOutputTokens != maxTokens || decoded.Temperature == nil || *decoded.Temperature != temperature || decoded.TopP == nil || *decoded.TopP != topP {
		t.Fatalf("generation controls = %#v", decoded)
	}
	if len(decoded.Stop) != 1 || decoded.Stop[0] != "END" {
		t.Fatalf("stop = %#v", decoded.Stop)
	}
}

func TestDecodeGenerateContentResponseMapsUsageAndStop(t *testing.T) {
	response, err := DecodeGenerateContentResponse([]byte(`{
		"responseId":"resp_1",
		"modelVersion":"gemini-test",
		"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}],
		"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2,"cachedContentTokenCount":1,"thoughtsTokenCount":3,"totalTokenCount":9}
	}`))
	if err != nil {
		t.Fatalf("DecodeGenerateContentResponse() error = %v", err)
	}
	if response.ID != "resp_1" || response.Model != "gemini-test" || response.StopReason != llm.StopReasonEndTurn {
		t.Fatalf("response = %#v", response)
	}
	if response.Usage.InputTokens != 4 || response.Usage.OutputTokens != 2 || response.Usage.CacheReadTokens != 1 || response.Usage.ReasoningTokens != 3 {
		t.Fatalf("usage = %#v", response.Usage)
	}
	if got := response.Content[0].(llm.TextBlock).Text; got != "hello" {
		t.Fatalf("text = %q", got)
	}
}

func TestDecodeGenerateContentResponsePreservesProviderMetadata(t *testing.T) {
	response, err := DecodeGenerateContentResponse([]byte(`{
		"responseId":"resp_grounded",
		"promptFeedback":{"blockReason":"OTHER"},
		"candidates":[{
			"content":{"role":"model","parts":[{"text":"grounded"}]},
			"finishReason":"STOP",
			"groundingMetadata":{"webSearchQueries":["example query"]}
		}],
		"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2,"trafficType":"ON_DEMAND"}
	}`))
	if err != nil {
		t.Fatalf("DecodeGenerateContentResponse() error = %v", err)
	}
	for _, key := range []string{"promptFeedback", "gemini.candidate", "gemini.usageMetadata"} {
		if len(response.Metadata[key]) == 0 {
			t.Fatalf("response metadata missing %q: %#v", key, response.Metadata)
		}
	}
}

func TestDecodeGenerateContentResponseMapsWebGroundingToCitations(t *testing.T) {
	response, err := DecodeGenerateContentResponse([]byte(`{
		"candidates":[{
			"content":{"role":"model","parts":[{"text":"Bangkok อากาศดี"}]},
			"finishReason":"STOP",
			"groundingMetadata":{
				"groundingChunks":[{"web":{"uri":"https://example.com/weather","title":"Weather"}}],
				"groundingSupports":[{
					"segment":{"partIndex":0,"startIndex":8,"endIndex":23,"text":"อากาศ"},
					"groundingChunkIndices":[0]
				}]
			}
		}]
	}`))
	if err != nil {
		t.Fatalf("DecodeGenerateContentResponse() error = %v", err)
	}
	text, ok := response.Content[0].(llm.TextBlock)
	if !ok || len(text.Citations) != 1 {
		t.Fatalf("text block = %#v", response.Content[0])
	}
	citation := text.Citations[0]
	if citation.StartIndex != 8 || citation.EndIndex != 13 || citation.URL != "https://example.com/weather" || citation.Title != "Weather" {
		t.Fatalf("citation = %#v", citation)
	}
	if len(response.Metadata) != 0 {
		t.Fatalf("metadata = %#v, want grounding consumed", response.Metadata)
	}
}

func TestDecodeGenerateContentResponseTreatsFunctionCallAsToolUse(t *testing.T) {
	response, err := DecodeGenerateContentResponse([]byte(`{
		"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"weather","args":{"city":"Bangkok"}}}]},"finishReason":"STOP"}]
	}`))
	if err != nil {
		t.Fatalf("DecodeGenerateContentResponse() error = %v", err)
	}
	if response.StopReason != llm.StopReasonToolUse {
		t.Fatalf("stop reason = %q", response.StopReason)
	}
	call, ok := response.Content[0].(llm.ToolCallBlock)
	if !ok || call.ID == "" || call.Name != "weather" {
		t.Fatalf("tool call = %#v", response.Content)
	}
}

func TestGenerateContentPathEscapesModel(t *testing.T) {
	path, err := GenerateContentPath("models/publisher/model")
	if err != nil {
		t.Fatalf("GenerateContentPath() error = %v", err)
	}
	if path != "/v1beta/models/publisher%2Fmodel:generateContent" {
		t.Fatalf("path = %q", path)
	}
}
