package gemini

import (
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeGenerateContentStreamEmitsReasoningAndSuppressesSignaturePart(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"responseId":"resp_1","modelVersion":"gemini-test","candidates":[{"content":{"role":"model","parts":[{"text":"plan ","thought":true}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"carefully","thought":true,"thoughtSignature":"opaque-provider-state"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"","thoughtSignature":"opaque-provider-state"}]}}]}`,
		``,
		`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"done"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":1,"thoughtsTokenCount":3}}`,
		``,
	}, "\n")

	var events []llm.StreamEvent
	if err := DecodeGenerateContentStream(strings.NewReader(stream), func(event llm.StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("DecodeGenerateContentStream() error = %v", err)
	}

	var summary, text strings.Builder
	var reasoningStarts, textStarts int
	for _, event := range events {
		switch event.Type {
		case llm.StreamEventContentStart:
			switch event.Block.(type) {
			case llm.ReasoningBlock:
				reasoningStarts++
			case llm.TextBlock:
				textStarts++
			}
		case llm.StreamEventReasoningDelta:
			summary.WriteString(event.ReasoningDelta)
		case llm.StreamEventTextDelta:
			text.WriteString(event.TextDelta)
		}
		if strings.Contains(event.ReasoningDelta, "opaque-provider-state") || strings.Contains(event.TextDelta, "opaque-provider-state") {
			t.Fatal("provider signature leaked into canonical stream")
		}
	}
	if reasoningStarts != 1 || textStarts != 1 || summary.String() != "plan carefully" || text.String() != "done" {
		t.Fatalf("decoded lifecycle reasoning=%d text=%d summary=%q output=%q events=%#v", reasoningStarts, textStarts, summary.String(), text.String(), events)
	}

	var sawReasoningUsage bool
	for _, event := range events {
		if event.Type == llm.StreamEventUsage && event.Usage != nil && event.Usage.ReasoningTokens == 3 {
			sawReasoningUsage = true
		}
	}
	if !sawReasoningUsage {
		t.Fatal("stream did not preserve Gemini reasoning token usage")
	}
}
