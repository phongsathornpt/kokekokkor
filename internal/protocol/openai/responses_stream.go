package openai

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/sse"
)

type responsesStreamBlock struct {
	kind        string
	outputIndex int
	itemID      string
	callID      string
	name        string
	text        strings.Builder
	arguments   strings.Builder
}

type ResponsesStreamEncoder struct {
	w                  io.Writer
	includeObfuscation bool
	id                 string
	model              string
	createdAt          int64
	sequence           int64
	started            bool
	stopped            bool
	latestUsage        llm.Usage
	output             []responseOutputItem
	blocks             map[int]*responsesStreamBlock
	allText            strings.Builder
	refusalMode        bool
	previousResponseID string
	conversationID     string
	store              bool
}

func NewResponsesStreamEncoder(w io.Writer, includeObfuscation bool) *ResponsesStreamEncoder {
	return &ResponsesStreamEncoder{
		w:                  w,
		includeObfuscation: includeObfuscation,
		blocks:             make(map[int]*responsesStreamBlock),
	}
}

func (e *ResponsesStreamEncoder) SetRefusalMode(enabled bool) {
	e.refusalMode = enabled
}

func (e *ResponsesStreamEncoder) SetState(previousResponseID, conversationID string, store bool) {
	e.previousResponseID = previousResponseID
	e.conversationID = conversationID
	e.store = store
}

func (e *ResponsesStreamEncoder) Encode(event llm.StreamEvent) error {
	switch event.Type {
	case llm.StreamEventResponseStart:
		return e.startResponse(event)
	case llm.StreamEventContentStart:
		return e.startContent(event)
	case llm.StreamEventTextDelta:
		return e.writeTextDelta(event)
	case llm.StreamEventToolCallStart:
		return e.startToolCall(event)
	case llm.StreamEventToolCallDelta:
		return e.writeToolCallDelta(event)
	case llm.StreamEventContentStop:
		return e.stopContent(event)
	case llm.StreamEventUsage:
		if event.Usage != nil {
			e.latestUsage = *event.Usage
		}
		return nil
	case llm.StreamEventResponseStop:
		return e.stopResponse(event)
	case llm.StreamEventReasoningDelta:
		return e.writeReasoningDelta(event)
	case llm.StreamEventError:
		return e.failResponse(event)
	default:
		return fmt.Errorf("unsupported canonical stream event %q", event.Type)
	}
}

func (e *ResponsesStreamEncoder) startResponse(event llm.StreamEvent) error {
	if e.started {
		return fmt.Errorf("Responses stream received duplicate response start")
	}
	e.started = true
	e.id = responsesStreamID(event.ResponseID)
	e.model = event.Model
	e.createdAt = time.Now().Unix()

	if err := e.write("response.created", map[string]any{
		"response": e.snapshot("in_progress", nil, false),
	}); err != nil {
		return err
	}
	return e.write("response.in_progress", map[string]any{
		"response": e.snapshot("in_progress", nil, false),
	})
}

func (e *ResponsesStreamEncoder) startContent(event llm.StreamEvent) error {
	if !e.started || e.stopped {
		return fmt.Errorf("content_start outside active Responses stream")
	}
	if _, exists := e.blocks[event.Index]; exists {
		return fmt.Errorf("content block %d already started", event.Index)
	}

	switch block := event.Block.(type) {
	case llm.ReasoningBlock:
		return e.startReasoningContent(event, block)
	case llm.TextBlock:
		state := &responsesStreamBlock{
			kind:        "text",
			outputIndex: len(e.output),
			itemID:      fmt.Sprintf("msg_gateway_%d", len(e.output)),
		}
		e.blocks[event.Index] = state
		item := responseOutputItem{
			ID:      state.itemID,
			Type:    "message",
			Status:  "in_progress",
			Role:    "assistant",
			Content: []responseOutputPart{},
		}
		e.output = append(e.output, item)
		if err := e.write("response.output_item.added", map[string]any{
			"output_index": state.outputIndex,
			"item":         item,
		}); err != nil {
			return err
		}
		part := responseOutputPart{Type: "output_text", Text: block.Text, Annotations: []any{}}
		if e.refusalMode {
			part = responseOutputPart{Type: "refusal", Refusal: block.Text}
		}
		if err := e.write("response.content_part.added", map[string]any{
			"item_id":       state.itemID,
			"output_index":  state.outputIndex,
			"content_index": 0,
			"part":          part,
		}); err != nil {
			return err
		}
		if block.Text != "" {
			state.text.WriteString(block.Text)
			if e.refusalMode {
				return e.writeRefusalDelta(state, block.Text)
			}
			e.allText.WriteString(block.Text)
			return e.writeOutputTextDelta(state, block.Text)
		}
		return nil
	default:
		return fmt.Errorf("content_start block %T is not supported by Responses stream encoder", event.Block)
	}
}

func (e *ResponsesStreamEncoder) writeTextDelta(event llm.StreamEvent) error {
	state, ok := e.blocks[event.Index]
	if !ok || state.kind != "text" {
		return fmt.Errorf("text_delta for unopened text block %d", event.Index)
	}
	state.text.WriteString(event.TextDelta)
	if e.refusalMode {
		return e.writeRefusalDelta(state, event.TextDelta)
	}
	e.allText.WriteString(event.TextDelta)
	return e.writeOutputTextDelta(state, event.TextDelta)
}

func (e *ResponsesStreamEncoder) startToolCall(event llm.StreamEvent) error {
	if !e.started || e.stopped {
		return fmt.Errorf("tool_call_start outside active Responses stream")
	}
	if _, exists := e.blocks[event.Index]; exists {
		return fmt.Errorf("content block %d already started", event.Index)
	}
	block, ok := event.Block.(llm.ToolCallBlock)
	if !ok {
		return fmt.Errorf("tool_call_start block is %T", event.Block)
	}
	state := &responsesStreamBlock{
		kind:        "tool",
		outputIndex: len(e.output),
		itemID:      fmt.Sprintf("fc_gateway_%d", len(e.output)),
		callID:      block.ID,
		name:        block.Name,
	}
	e.blocks[event.Index] = state
	item := responseOutputItem{
		ID:        state.itemID,
		Type:      "function_call",
		Status:    "in_progress",
		CallID:    state.callID,
		Name:      state.name,
		Arguments: "",
	}
	e.output = append(e.output, item)
	return e.write("response.output_item.added", map[string]any{
		"output_index": state.outputIndex,
		"item":         item,
	})
}

func (e *ResponsesStreamEncoder) writeToolCallDelta(event llm.StreamEvent) error {
	state, ok := e.blocks[event.Index]
	if !ok || state.kind != "tool" {
		return fmt.Errorf("tool_call_delta for unopened tool block %d", event.Index)
	}
	if event.ToolCallDelta == nil {
		return fmt.Errorf("tool_call_delta is missing delta payload")
	}
	if event.ToolCallDelta.ArgumentsDelta == "" {
		return nil
	}
	state.arguments.WriteString(event.ToolCallDelta.ArgumentsDelta)
	fields := map[string]any{
		"item_id":      state.itemID,
		"output_index": state.outputIndex,
		"delta":        event.ToolCallDelta.ArgumentsDelta,
	}
	e.addObfuscation(fields)
	return e.write("response.function_call_arguments.delta", fields)
}

func (e *ResponsesStreamEncoder) stopContent(event llm.StreamEvent) error {
	state, ok := e.blocks[event.Index]
	if !ok {
		return fmt.Errorf("content_stop for unopened block %d", event.Index)
	}
	delete(e.blocks, event.Index)

	switch state.kind {
	case "reasoning":
		return e.stopReasoningContent(state)
	case "text":
		text := state.text.String()
		if e.refusalMode {
			if err := e.write("response.refusal.done", map[string]any{
				"item_id":       state.itemID,
				"output_index":  state.outputIndex,
				"content_index": 0,
				"refusal":       text,
			}); err != nil {
				return err
			}
			part := responseOutputPart{Type: "refusal", Refusal: text}
			if err := e.write("response.content_part.done", map[string]any{
				"item_id":       state.itemID,
				"output_index":  state.outputIndex,
				"content_index": 0,
				"part":          part,
			}); err != nil {
				return err
			}
			e.output[state.outputIndex].Status = "completed"
			e.output[state.outputIndex].Content = []responseOutputPart{part}
			return e.write("response.output_item.done", map[string]any{
				"output_index": state.outputIndex,
				"item":         e.output[state.outputIndex],
			})
		}
		if err := e.write("response.output_text.done", map[string]any{
			"item_id":       state.itemID,
			"output_index":  state.outputIndex,
			"content_index": 0,
			"text":          text,
		}); err != nil {
			return err
		}
		part := responseOutputPart{Type: "output_text", Text: text, Annotations: []any{}}
		if err := e.write("response.content_part.done", map[string]any{
			"item_id":       state.itemID,
			"output_index":  state.outputIndex,
			"content_index": 0,
			"part":          part,
		}); err != nil {
			return err
		}
		e.output[state.outputIndex].Status = "completed"
		e.output[state.outputIndex].Content = []responseOutputPart{part}
		return e.write("response.output_item.done", map[string]any{
			"output_index": state.outputIndex,
			"item":         e.output[state.outputIndex],
		})

	case "tool":
		arguments := state.arguments.String()
		if arguments == "" {
			arguments = "{}"
		}
		if err := e.write("response.function_call_arguments.done", map[string]any{
			"item_id":      state.itemID,
			"output_index": state.outputIndex,
			"arguments":    arguments,
			"name":         state.name,
		}); err != nil {
			return err
		}
		e.output[state.outputIndex].Status = "completed"
		e.output[state.outputIndex].Arguments = arguments
		return e.write("response.output_item.done", map[string]any{
			"output_index": state.outputIndex,
			"item":         e.output[state.outputIndex],
		})
	default:
		return fmt.Errorf("unknown Responses stream block kind %q", state.kind)
	}
}

func (e *ResponsesStreamEncoder) stopResponse(event llm.StreamEvent) error {
	if !e.started || e.stopped {
		return fmt.Errorf("response_stop outside active Responses stream")
	}
	if len(e.blocks) != 0 {
		return fmt.Errorf("Responses stream stopped with %d open content blocks", len(e.blocks))
	}
	e.stopped = true

	switch event.StopReason {
	case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse:
		return e.write("response.completed", map[string]any{
			"response": e.snapshot("completed", nil, true),
		})
	case llm.StopReasonContentBlock:
		if !e.refusalMode {
			return fmt.Errorf("content-block stop requires refusal mode")
		}
		return e.write("response.completed", map[string]any{
			"response": e.snapshot("completed", nil, true),
		})
	case llm.StopReasonMaxTokens:
		return e.write("response.incomplete", map[string]any{
			"response": e.snapshot("incomplete", map[string]string{"reason": "max_output_tokens"}, false),
		})
	default:
		return fmt.Errorf("unsupported canonical stop reason %q for Responses stream", event.StopReason)
	}
}

func (e *ResponsesStreamEncoder) writeRefusalDelta(state *responsesStreamBlock, delta string) error {
	fields := map[string]any{
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"content_index": 0,
		"delta":         delta,
	}
	e.addObfuscation(fields)
	return e.write("response.refusal.delta", fields)
}

func (e *ResponsesStreamEncoder) writeOutputTextDelta(state *responsesStreamBlock, delta string) error {
	fields := map[string]any{
		"item_id":       state.itemID,
		"output_index":  state.outputIndex,
		"content_index": 0,
		"delta":         delta,
		"logprobs":      []any{},
	}
	e.addObfuscation(fields)
	return e.write("response.output_text.delta", fields)
}

func (e *ResponsesStreamEncoder) write(eventType string, fields map[string]any) error {
	fields["type"] = eventType
	fields["sequence_number"] = e.sequence
	e.sequence++
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return sse.Write(e.w, sse.Event{Name: eventType, Data: data})
}

func (e *ResponsesStreamEncoder) snapshot(status string, incomplete any, completed bool) responsesStreamSnapshot {
	var completedAt *int64
	if completed {
		now := time.Now().Unix()
		completedAt = &now
	}
	usage := encodeResponsesStreamUsage(e.latestUsage)
	var previousResponseID any
	if e.previousResponseID != "" {
		previousResponseID = e.previousResponseID
	}
	var conversation *responseConversation
	if e.conversationID != "" {
		conversation = &responseConversation{ID: e.conversationID}
	}
	return responsesStreamSnapshot{
		ID:                 e.id,
		Object:             "response",
		CreatedAt:          e.createdAt,
		CompletedAt:        completedAt,
		Status:             status,
		Error:              nil,
		IncompleteDetails:  incomplete,
		Model:              e.model,
		Output:             append([]responseOutputItem(nil), e.output...),
		OutputText:         e.allText.String(),
		Usage:              &usage,
		ParallelToolCalls:  true,
		PreviousResponseID: previousResponseID,
		Conversation:       conversation,
		Reasoning:          map[string]any{"effort": nil, "summary": nil},
		Store:              e.store,
		Temperature:        1,
		Text:               map[string]any{"format": map[string]any{"type": "text"}},
		ToolChoice:         "auto",
		Tools:              []any{},
		TopP:               1,
		Truncation:         "disabled",
		Metadata:           map[string]string{},
	}
}

func (e *ResponsesStreamEncoder) addObfuscation(fields map[string]any) {
	if !e.includeObfuscation {
		return
	}
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return
	}
	fields["obfuscation"] = base64.RawURLEncoding.EncodeToString(raw[:])
}

func responsesStreamID(upstreamID string) string {
	if strings.HasPrefix(upstreamID, "resp_") {
		return upstreamID
	}
	if upstreamID == "" {
		return "resp_gateway"
	}
	return "resp_" + upstreamID
}

func encodeResponsesStreamUsage(usage llm.Usage) responseUsage {
	return responseUsage{
		InputTokens: usage.InputTokens,
		InputTokensDetails: responseInputTokenDetails{
			CachedTokens:     usage.CacheReadTokens,
			CacheWriteTokens: usage.CacheWriteTokens,
		},
		OutputTokens: usage.OutputTokens,
		OutputTokensDetail: responseOutputTokenDetails{
			ReasoningTokens: usage.ReasoningTokens,
		},
		TotalTokens: usage.InputTokens + usage.OutputTokens,
	}
}
