package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

// RealtimeServerEncoder encodes the portable text-only canonical stream into
// OpenAI Realtime WebSocket server events.
type RealtimeServerEncoder struct {
	sequence    uint64
	responseID  string
	itemID      string
	model       string
	text        strings.Builder
	usage       llm.Usage
	started     bool
	contentOpen bool
	stopped     bool
}

func NewRealtimeServerEncoder() *RealtimeServerEncoder {
	return &RealtimeServerEncoder{}
}

func (e *RealtimeServerEncoder) Encode(event llm.StreamEvent) ([][]byte, error) {
	switch event.Type {
	case llm.StreamEventResponseStart:
		return e.startResponse(event)
	case llm.StreamEventContentStart:
		return e.startContent(event)
	case llm.StreamEventTextDelta:
		return e.textDelta(event)
	case llm.StreamEventUsage:
		if event.Usage != nil {
			e.usage = *event.Usage
		}
		return nil, nil
	case llm.StreamEventContentStop:
		return e.stopContent(event)
	case llm.StreamEventResponseStop:
		return e.stopResponse(event)
	default:
		return nil, fmt.Errorf("OpenAI Realtime server encoder: unsupported canonical event %q", event.Type)
	}
}

func (e *RealtimeServerEncoder) startResponse(event llm.StreamEvent) ([][]byte, error) {
	if e.started && !e.stopped {
		return nil, fmt.Errorf("OpenAI Realtime server encoder received duplicate response start")
	}
	e.reset()
	e.started = true
	e.model = event.Model
	e.responseID = event.ResponseID
	if e.responseID == "" {
		e.responseID = e.nextID("resp")
	}
	e.itemID = e.nextID("item")
	return e.marshalEvents(map[string]any{
		"type":     "response.created",
		"event_id": e.nextID("event"),
		"response": e.responseSnapshot("in_progress", nil, nil),
	})
}

func (e *RealtimeServerEncoder) startContent(event llm.StreamEvent) ([][]byte, error) {
	if !e.started || e.stopped || e.contentOpen {
		return nil, fmt.Errorf("content_start outside valid OpenAI Realtime response state")
	}
	if _, ok := event.Block.(llm.TextBlock); !ok {
		return nil, fmt.Errorf("OpenAI Realtime text bridge cannot encode content block %T", event.Block)
	}
	e.contentOpen = true
	item := e.itemSnapshot("in_progress", "")
	return e.marshalEvents(
		map[string]any{
			"type":         "response.output_item.added",
			"event_id":     e.nextID("event"),
			"response_id":  e.responseID,
			"output_index": 0,
			"item":         item,
		},
		map[string]any{
			"type":          "response.content_part.added",
			"event_id":      e.nextID("event"),
			"response_id":   e.responseID,
			"item_id":       e.itemID,
			"output_index":  0,
			"content_index": 0,
			"part":          map[string]any{"type": "text", "text": ""},
		},
	)
}

func (e *RealtimeServerEncoder) textDelta(event llm.StreamEvent) ([][]byte, error) {
	if !e.contentOpen || e.stopped {
		return nil, fmt.Errorf("text_delta outside active OpenAI Realtime text content")
	}
	e.text.WriteString(event.TextDelta)
	return e.marshalEvents(map[string]any{
		"type":          "response.output_text.delta",
		"event_id":      e.nextID("event"),
		"response_id":   e.responseID,
		"item_id":       e.itemID,
		"output_index":  0,
		"content_index": 0,
		"delta":         event.TextDelta,
	})
}

func (e *RealtimeServerEncoder) stopContent(event llm.StreamEvent) ([][]byte, error) {
	if !e.contentOpen || e.stopped {
		return nil, fmt.Errorf("content_stop outside active OpenAI Realtime text content")
	}
	e.contentOpen = false
	text := e.text.String()
	part := map[string]any{"type": "text", "text": text}
	item := e.itemSnapshot("completed", text)
	return e.marshalEvents(
		map[string]any{
			"type":          "response.output_text.done",
			"event_id":      e.nextID("event"),
			"response_id":   e.responseID,
			"item_id":       e.itemID,
			"output_index":  0,
			"content_index": 0,
			"text":          text,
		},
		map[string]any{
			"type":          "response.content_part.done",
			"event_id":      e.nextID("event"),
			"response_id":   e.responseID,
			"item_id":       e.itemID,
			"output_index":  0,
			"content_index": 0,
			"part":          part,
		},
		map[string]any{
			"type":         "response.output_item.done",
			"event_id":     e.nextID("event"),
			"response_id":  e.responseID,
			"output_index": 0,
			"item":         item,
		},
	)
}

func (e *RealtimeServerEncoder) stopResponse(event llm.StreamEvent) ([][]byte, error) {
	if !e.started || e.stopped || e.contentOpen {
		return nil, fmt.Errorf("response_stop outside valid OpenAI Realtime response state")
	}
	if event.StopReason != llm.StopReasonEndTurn && event.StopReason != llm.StopReasonStopSequence {
		return nil, fmt.Errorf("OpenAI Realtime text bridge cannot encode stop reason %q", event.StopReason)
	}
	e.stopped = true
	return e.marshalEvents(map[string]any{
		"type":     "response.done",
		"event_id": e.nextID("event"),
		"response": e.responseSnapshot("completed", nil, e.itemSnapshot("completed", e.text.String())),
	})
}

func (e *RealtimeServerEncoder) responseSnapshot(status string, statusDetails any, item any) map[string]any {
	output := []any{}
	if item != nil {
		output = append(output, item)
	}
	usage := any(nil)
	if e.usage.InputTokens != 0 || e.usage.OutputTokens != 0 || e.usage.CacheReadTokens != 0 || e.usage.ReasoningTokens != 0 {
		usage = map[string]any{
			"total_tokens":  e.usage.InputTokens + e.usage.OutputTokens,
			"input_tokens":  e.usage.InputTokens,
			"output_tokens": e.usage.OutputTokens,
			"input_token_details": map[string]any{
				"text_tokens":   e.usage.InputTokens,
				"audio_tokens":  0,
				"image_tokens":  0,
				"cached_tokens": e.usage.CacheReadTokens,
			},
			"output_token_details": map[string]any{
				"text_tokens":  e.usage.OutputTokens,
				"audio_tokens": 0,
			},
		}
	}
	return map[string]any{
		"object":            "realtime.response",
		"id":                e.responseID,
		"status":            status,
		"status_details":    statusDetails,
		"output":            output,
		"conversation_id":   nil,
		"output_modalities": []string{"text"},
		"max_output_tokens": "inf",
		"usage":             usage,
		"metadata":          nil,
	}
}

func (e *RealtimeServerEncoder) itemSnapshot(status, text string) map[string]any {
	content := []any{}
	if status == "completed" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	return map[string]any{
		"id":      e.itemID,
		"object":  "realtime.item",
		"type":    "message",
		"status":  status,
		"role":    "assistant",
		"content": content,
	}
}

func (e *RealtimeServerEncoder) marshalEvents(events ...map[string]any) ([][]byte, error) {
	encoded := make([][]byte, 0, len(events))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return nil, fmt.Errorf("encode OpenAI Realtime server event: %w", err)
		}
		encoded = append(encoded, data)
	}
	return encoded, nil
}

func (e *RealtimeServerEncoder) nextID(prefix string) string {
	e.sequence++
	return fmt.Sprintf("%s_gateway_%d", prefix, e.sequence)
}

func (e *RealtimeServerEncoder) reset() {
	e.responseID = ""
	e.itemID = ""
	e.model = ""
	e.text.Reset()
	e.usage = llm.Usage{}
	e.contentOpen = false
	e.stopped = false
}
