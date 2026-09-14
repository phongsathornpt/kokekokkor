package openai

import (
	"encoding/json"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeResponsesReasoningConfig(t *testing.T) {
	request, err := DecodeResponsesRequest([]byte(`{
		"model":"gpt-model",
		"input":"hello",
		"reasoning":{"effort":"high","summary":"auto"}
	}`))
	if err != nil {
		t.Fatalf("DecodeResponsesRequest() error = %v", err)
	}
	if request.Reasoning == nil || !request.Reasoning.Enabled || request.Reasoning.Effort != "high" || request.Reasoning.Summary != "auto" {
		t.Fatalf("Reasoning = %#v", request.Reasoning)
	}
}

func TestEncodeResponsesReasoningSummary(t *testing.T) {
	encoded, err := EncodeResponsesResponse(llm.Response{
		ID:         "resp_1",
		Model:      "provider-model",
		StopReason: llm.StopReasonEndTurn,
		Content: []llm.ContentBlock{
			llm.ReasoningBlock{Text: "condensed reasoning", Signature: "provider-signature"},
			llm.TextBlock{Text: "answer"},
		},
		Usage: llm.Usage{InputTokens: 2, OutputTokens: 4, ReasoningTokens: 3},
	})
	if err != nil {
		t.Fatalf("EncodeResponsesResponse() error = %v", err)
	}
	var wire responseObject
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(wire.Output) != 2 {
		t.Fatalf("Output = %#v", wire.Output)
	}
	if wire.Output[0].Type != "reasoning" || len(wire.Output[0].Summary) != 1 || wire.Output[0].Summary[0].Text != "condensed reasoning" {
		t.Fatalf("reasoning output = %#v", wire.Output[0])
	}
	if wire.OutputText != "answer" || wire.Usage.OutputTokensDetail.ReasoningTokens != 3 {
		t.Fatalf("response = %#v", wire)
	}
}
