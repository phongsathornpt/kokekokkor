package gemini

import (
	"encoding/json"
	"testing"
)

func TestGenerateContentThinkingControlsRoundTrip(t *testing.T) {
	input := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{"thinkingConfig":{"includeThoughts":true,"thinkingLevel":"HIGH"}}
	}`)
	request, err := DecodeGenerateContentRequest(input)
	if err != nil {
		t.Fatalf("DecodeGenerateContentRequest() error = %v", err)
	}
	if request.Reasoning == nil || request.Reasoning.Effort != "high" || request.Reasoning.Summary != "auto" {
		t.Fatalf("Reasoning = %#v", request.Reasoning)
	}

	encoded, err := EncodeGenerateContentRequest(request)
	if err != nil {
		t.Fatalf("EncodeGenerateContentRequest() error = %v", err)
	}
	var wire generateContentRequest
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if wire.GenerationConfig == nil || wire.GenerationConfig.ThinkingConfig == nil {
		t.Fatalf("GenerationConfig = %#v", wire.GenerationConfig)
	}
	thinking := wire.GenerationConfig.ThinkingConfig
	if thinking.ThinkingLevel != "HIGH" || thinking.IncludeThoughts == nil || !*thinking.IncludeThoughts {
		t.Fatalf("ThinkingConfig = %#v", thinking)
	}
}

func TestEncodeGenerateContentDisablesThinkingWithNone(t *testing.T) {
	request, err := DecodeGenerateContentRequest([]byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`))
	if err != nil {
		t.Fatalf("DecodeGenerateContentRequest() error = %v", err)
	}
	request.Reasoning = &llm.ReasoningConfig{Enabled: false, Effort: "none", Summary: "none"}
	encoded, err := EncodeGenerateContentRequest(request)
	if err != nil {
		t.Fatalf("EncodeGenerateContentRequest() error = %v", err)
	}
	var wire generateContentRequest
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	thinking := wire.GenerationConfig.ThinkingConfig
	if thinking.ThinkingBudget == nil || *thinking.ThinkingBudget != 0 {
		t.Fatalf("ThinkingConfig = %#v", thinking)
	}
}
