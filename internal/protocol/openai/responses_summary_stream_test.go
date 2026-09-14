package openai

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/sse"
)

func TestResponsesStreamEncoderReasoningSummaryLifecycle(t *testing.T) {
	var buffer bytes.Buffer
	encoder := NewResponsesStreamEncoder(&buffer, false)
	usage := llm.Usage{InputTokens: 4, OutputTokens: 3, ReasoningTokens: 2}
	events := []llm.StreamEvent{
		{Type: llm.StreamEventResponseStart, ResponseID: "msg_reasoning", Model: "provider-model"},
		{Type: llm.StreamEventContentStart, Index: 0, Block: llm.ReasoningBlock{}},
		{Type: llm.StreamEventReasoningDelta, Index: 0, ReasoningDelta: "plan"},
		{Type: llm.StreamEventReasoningDelta, Index: 0, ReasoningDelta: "ning"},
		{Type: llm.StreamEventContentStop, Index: 0},
		{Type: llm.StreamEventContentStart, Index: 1, Block: llm.TextBlock{}},
		{Type: llm.StreamEventTextDelta, Index: 1, TextDelta: "done"},
		{Type: llm.StreamEventContentStop, Index: 1},
		{Type: llm.StreamEventUsage, Usage: &usage},
		{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonEndTurn},
	}
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			t.Fatalf("Encode(%q) error = %v", event.Type, err)
		}
	}

	var names []string
	var summaryDone struct {
		Text string `json:"text"`
	}
	var terminal map[string]json.RawMessage
	if err := sse.Decode(strings.NewReader(buffer.String()), func(event sse.Event) error {
		names = append(names, event.Name)
		switch event.Name {
		case "response.reasoning_summary_text.done":
			return json.Unmarshal(event.Data, &summaryDone)
		case "response.completed":
			return json.Unmarshal(event.Data, &terminal)
		default:
			return nil
		}
	}); err != nil {
		t.Fatalf("decode generated SSE: %v", err)
	}

	for _, want := range []string{
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_text.delta",
		"response.reasoning_summary_text.done",
		"response.reasoning_summary_part.done",
	} {
		if !containsString(names, want) {
			t.Fatalf("stream missing %s: %#v", want, names)
		}
	}
	if summaryDone.Text != "planning" {
		t.Fatalf("summary done text = %q, want planning", summaryDone.Text)
	}

	var response struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Summary []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"summary"`
		} `json:"output"`
		Usage struct {
			OutputTokensDetails struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(terminal["response"], &response); err != nil {
		t.Fatalf("decode terminal response: %v", err)
	}
	if response.OutputText != "done" {
		t.Fatalf("output_text = %q, want done", response.OutputText)
	}
	if len(response.Output) < 2 || response.Output[0].Type != "reasoning" || len(response.Output[0].Summary) != 1 || response.Output[0].Summary[0].Text != "planning" {
		t.Fatalf("terminal output = %#v", response.Output)
	}
	if response.Usage.OutputTokensDetails.ReasoningTokens != 2 {
		t.Fatalf("reasoning tokens = %d, want 2", response.Usage.OutputTokensDetails.ReasoningTokens)
	}
}
