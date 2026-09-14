package openai

import (
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func (e *ResponsesStreamEncoder) startReasoningContent(event llm.StreamEvent, block llm.ReasoningBlock) error {
	if block.Signature != "" || block.RedactedData != "" {
		return fmt.Errorf("reasoning block contains provider-only state")
	}
	state := &responsesStreamBlock{
		kind:        "reasoning",
		outputIndex: len(e.output),
		itemID:      fmt.Sprintf("rs_gateway_%d", len(e.output)),
	}
	e.blocks[event.Index] = state
	item := responseOutputItem{
		ID:      state.itemID,
		Type:    "reasoning",
		Status:  "in_progress",
		Summary: []responseSummaryPart{},
	}
	e.output = append(e.output, item)
	if err := e.write("response.output_item.added", map[string]any{
		"output_index": state.outputIndex,
		"item":         item,
	}); err != nil {
		return err
	}
	if err := e.write("response.reasoning_summary_part.added", map[string]any{
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"summary_index": 0,
		"part":          responseSummaryPart{Type: "summary_text", Text: ""},
	}); err != nil {
		return err
	}
	if block.Text == "" {
		return nil
	}
	state.text.WriteString(block.Text)
	return e.writeReasoningSummaryDelta(state, block.Text)
}

func (e *ResponsesStreamEncoder) writeReasoningDelta(event llm.StreamEvent) error {
	state, ok := e.blocks[event.Index]
	if !ok || state.kind != "reasoning" {
		return fmt.Errorf("reasoning_delta for unopened reasoning block %d", event.Index)
	}
	if event.ReasoningDelta == "" {
		return nil
	}
	state.text.WriteString(event.ReasoningDelta)
	return e.writeReasoningSummaryDelta(state, event.ReasoningDelta)
}

func (e *ResponsesStreamEncoder) stopReasoningContent(state *responsesStreamBlock) error {
	text := state.text.String()
	if err := e.write("response.reasoning_summary_text.done", map[string]any{
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"summary_index": 0,
		"text":          text,
	}); err != nil {
		return err
	}
	part := responseSummaryPart{Type: "summary_text", Text: text}
	if err := e.write("response.reasoning_summary_part.done", map[string]any{
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"summary_index": 0,
		"part":          part,
	}); err != nil {
		return err
	}
	e.output[state.outputIndex].Status = "completed"
	e.output[state.outputIndex].Summary = []responseSummaryPart{part}
	return e.write("response.output_item.done", map[string]any{
		"output_index": state.outputIndex,
		"item":         e.output[state.outputIndex],
	})
}

func (e *ResponsesStreamEncoder) writeReasoningSummaryDelta(state *responsesStreamBlock, delta string) error {
	fields := map[string]any{
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"summary_index": 0,
		"delta":         delta,
	}
	e.addObfuscation(fields)
	return e.write("response.reasoning_summary_text.delta", fields)
}
