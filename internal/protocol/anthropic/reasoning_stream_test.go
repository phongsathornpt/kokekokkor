package anthropic

import (
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeMessagesStreamEmitsReasoningSummaryWithoutSignature(t *testing.T) {
	stream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude","usage":{"input_tokens":2,"output_tokens":0}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"plan "}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"carefully"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"opaque-provider-state"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"done"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":4}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	var events []llm.StreamEvent
	if err := DecodeMessagesStream(strings.NewReader(stream), func(event llm.StreamEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("DecodeMessagesStream() error = %v", err)
	}

	var summary strings.Builder
	var reasoningStarts, reasoningStops int
	for _, event := range events {
		switch event.Type {
		case llm.StreamEventContentStart:
			if _, ok := event.Block.(llm.ReasoningBlock); ok {
				reasoningStarts++
			}
		case llm.StreamEventReasoningDelta:
			summary.WriteString(event.ReasoningDelta)
		case llm.StreamEventContentStop:
			if event.Index == 0 {
				reasoningStops++
			}
		}
		if strings.Contains(event.ReasoningDelta, "opaque-provider-state") {
			t.Fatal("provider signature leaked into canonical stream")
		}
	}
	if reasoningStarts != 1 || reasoningStops != 1 || summary.String() != "plan carefully" {
		t.Fatalf("reasoning lifecycle starts=%d stops=%d summary=%q events=%#v", reasoningStarts, reasoningStops, summary.String(), events)
	}
}

func TestDecodeMessagesStreamSuppressesSignatureOnlyThinkingBlock(t *testing.T) {
	stream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_2","model":"claude","usage":{"input_tokens":1,"output_tokens":0}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"opaque"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	if err := DecodeMessagesStream(strings.NewReader(stream), func(event llm.StreamEvent) error {
		if event.Type == llm.StreamEventReasoningDelta {
			t.Fatalf("signature-only block emitted reasoning delta: %#v", event)
		}
		if event.Type == llm.StreamEventContentStart {
			if _, ok := event.Block.(llm.ReasoningBlock); ok {
				t.Fatalf("signature-only block emitted visible reasoning content: %#v", event)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("DecodeMessagesStream() error = %v", err)
	}
}
