package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

// RealtimeServerEncoder encodes the portable text/function/audio canonical
// stream into OpenAI Realtime WebSocket server events.
type RealtimeServerEncoder struct {
	sequence     uint64
	responseID   string
	model        string
	text         strings.Builder
	arguments    strings.Builder
	usage        llm.Usage
	started      bool
	contentOpen  bool
	stopped      bool
	activeKind   string
	activeIndex  int
	activeItemID string
	activeCallID string
	activeName   string
	output       []any
}

func NewRealtimeServerEncoder() *RealtimeServerEncoder { return &RealtimeServerEncoder{} }

func (e *RealtimeServerEncoder) Encode(event llm.StreamEvent) ([][]byte, error) {
	switch event.Type {
	case llm.StreamEventResponseStart:
		return e.startResponse(event)
	case llm.StreamEventContentStart, llm.StreamEventToolCallStart:
		return e.startContent(event)
	case llm.StreamEventTextDelta:
		return e.textDelta(event)
	case llm.StreamEventAudioDelta:
		return e.audioDelta(event)
	case llm.StreamEventToolCallDelta:
		return e.toolCallDelta(event)
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
	return e.marshalEvents(map[string]any{"type": "response.created", "event_id": e.nextID("event"), "response": e.responseSnapshot("in_progress", nil)})
}

func (e *RealtimeServerEncoder) startContent(event llm.StreamEvent) ([][]byte, error) {
	if !e.started || e.stopped || e.contentOpen {
		return nil, fmt.Errorf("content start outside valid OpenAI Realtime response state")
	}
	e.contentOpen = true
	e.activeIndex = event.Index
	e.activeItemID = e.nextID("item")
	outputIndex := len(e.output)

	switch block := event.Block.(type) {
	case llm.TextBlock:
		e.activeKind = "text"
		e.text.Reset()
		item := e.messageItemSnapshot("in_progress", map[string]any{"type": "text", "text": ""}, false)
		return e.marshalEvents(
			map[string]any{"type": "response.output_item.added", "event_id": e.nextID("event"), "response_id": e.responseID, "output_index": outputIndex, "item": item},
			map[string]any{"type": "response.content_part.added", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": outputIndex, "content_index": 0, "part": map[string]any{"type": "text", "text": ""}},
		)
	case llm.AudioBlock:
		if block.MediaType != "audio/pcm;rate=24000" {
			e.contentOpen = false
			return nil, fmt.Errorf("OpenAI Realtime audio bridge requires PCM 24kHz output, got %q", block.MediaType)
		}
		e.activeKind = "audio"
		part := map[string]any{"type": "audio", "transcript": ""}
		item := e.messageItemSnapshot("in_progress", part, false)
		return e.marshalEvents(
			map[string]any{"type": "response.output_item.added", "event_id": e.nextID("event"), "response_id": e.responseID, "output_index": outputIndex, "item": item},
			map[string]any{"type": "response.content_part.added", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": outputIndex, "content_index": 0, "part": part},
		)
	case llm.ToolCallBlock:
		if block.ID == "" || block.Name == "" {
			return nil, fmt.Errorf("OpenAI Realtime function call requires id and name")
		}
		e.activeKind = "tool"
		e.activeCallID = block.ID
		e.activeName = block.Name
		e.arguments.Reset()
		return e.marshalEvents(map[string]any{"type": "response.output_item.added", "event_id": e.nextID("event"), "response_id": e.responseID, "output_index": outputIndex, "item": e.toolItemSnapshot("in_progress", "")})
	default:
		e.contentOpen = false
		return nil, fmt.Errorf("OpenAI Realtime bridge cannot encode content block %T", event.Block)
	}
}

func (e *RealtimeServerEncoder) textDelta(event llm.StreamEvent) ([][]byte, error) {
	if !e.contentOpen || e.stopped || e.activeKind != "text" || event.Index != e.activeIndex {
		return nil, fmt.Errorf("text_delta outside active OpenAI Realtime text content")
	}
	e.text.WriteString(event.TextDelta)
	return e.marshalEvents(map[string]any{"type": "response.output_text.delta", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": len(e.output), "content_index": 0, "delta": event.TextDelta})
}

func (e *RealtimeServerEncoder) audioDelta(event llm.StreamEvent) ([][]byte, error) {
	if !e.contentOpen || e.stopped || e.activeKind != "audio" || event.Index != e.activeIndex {
		return nil, fmt.Errorf("audio_delta outside active OpenAI Realtime audio content")
	}
	if event.AudioMediaType != "" && event.AudioMediaType != "audio/pcm;rate=24000" {
		return nil, fmt.Errorf("OpenAI Realtime audio bridge cannot encode %q", event.AudioMediaType)
	}
	return e.marshalEvents(map[string]any{"type": "response.output_audio.delta", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": len(e.output), "content_index": 0, "delta": event.AudioDelta})
}

func (e *RealtimeServerEncoder) toolCallDelta(event llm.StreamEvent) ([][]byte, error) {
	if !e.contentOpen || e.stopped || e.activeKind != "tool" || event.Index != e.activeIndex {
		return nil, fmt.Errorf("tool_call_delta outside active OpenAI Realtime function call")
	}
	if event.ToolCallDelta == nil {
		return nil, fmt.Errorf("tool_call_delta is missing payload")
	}
	e.arguments.WriteString(event.ToolCallDelta.ArgumentsDelta)
	return e.marshalEvents(map[string]any{"type": "response.function_call_arguments.delta", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": len(e.output), "call_id": e.activeCallID, "delta": event.ToolCallDelta.ArgumentsDelta})
}

func (e *RealtimeServerEncoder) stopContent(event llm.StreamEvent) ([][]byte, error) {
	if !e.contentOpen || e.stopped || event.Index != e.activeIndex {
		return nil, fmt.Errorf("content_stop outside active OpenAI Realtime content")
	}
	e.contentOpen = false
	outputIndex := len(e.output)

	switch e.activeKind {
	case "text":
		text := e.text.String()
		part := map[string]any{"type": "text", "text": text}
		item := e.messageItemSnapshot("completed", part, true)
		e.output = append(e.output, item)
		return e.marshalEvents(
			map[string]any{"type": "response.output_text.done", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": outputIndex, "content_index": 0, "text": text},
			map[string]any{"type": "response.content_part.done", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": outputIndex, "content_index": 0, "part": part},
			map[string]any{"type": "response.output_item.done", "event_id": e.nextID("event"), "response_id": e.responseID, "output_index": outputIndex, "item": item},
		)
	case "audio":
		part := map[string]any{"type": "audio", "transcript": ""}
		item := e.messageItemSnapshot("completed", part, true)
		e.output = append(e.output, item)
		return e.marshalEvents(
			map[string]any{"type": "response.output_audio.done", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": outputIndex, "content_index": 0},
			map[string]any{"type": "response.content_part.done", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": outputIndex, "content_index": 0, "part": part},
			map[string]any{"type": "response.output_item.done", "event_id": e.nextID("event"), "response_id": e.responseID, "output_index": outputIndex, "item": item},
		)
	case "tool":
		arguments := e.arguments.String()
		if arguments == "" {
			arguments = "{}"
		}
		item := e.toolItemSnapshot("completed", arguments)
		e.output = append(e.output, item)
		return e.marshalEvents(
			map[string]any{"type": "response.function_call_arguments.done", "event_id": e.nextID("event"), "response_id": e.responseID, "item_id": e.activeItemID, "output_index": outputIndex, "call_id": e.activeCallID, "name": e.activeName, "arguments": arguments},
			map[string]any{"type": "response.output_item.done", "event_id": e.nextID("event"), "response_id": e.responseID, "output_index": outputIndex, "item": item},
		)
	default:
		return nil, fmt.Errorf("unknown OpenAI Realtime content kind %q", e.activeKind)
	}
}

func (e *RealtimeServerEncoder) stopResponse(event llm.StreamEvent) ([][]byte, error) {
	if !e.started || e.stopped || e.contentOpen {
		return nil, fmt.Errorf("response_stop outside valid OpenAI Realtime response state")
	}
	switch event.StopReason {
	case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse:
	default:
		return nil, fmt.Errorf("OpenAI Realtime bridge cannot encode stop reason %q", event.StopReason)
	}
	e.stopped = true
	return e.marshalEvents(map[string]any{"type": "response.done", "event_id": e.nextID("event"), "response": e.responseSnapshot("completed", nil)})
}

func (e *RealtimeServerEncoder) responseSnapshot(status string, statusDetails any) map[string]any {
	usage := any(nil)
	if e.usage.InputTokens != 0 || e.usage.OutputTokens != 0 || e.usage.CacheReadTokens != 0 || e.usage.ReasoningTokens != 0 {
		usage = map[string]any{"total_tokens": e.usage.InputTokens + e.usage.OutputTokens, "input_tokens": e.usage.InputTokens, "output_tokens": e.usage.OutputTokens, "input_token_details": map[string]any{"text_tokens": e.usage.InputTokens, "audio_tokens": 0, "image_tokens": 0, "cached_tokens": e.usage.CacheReadTokens}, "output_token_details": map[string]any{"text_tokens": e.usage.OutputTokens, "audio_tokens": 0}}
	}
	modality := "text"
	if e.activeKind == "audio" {
		modality = "audio"
	}
	return map[string]any{"object": "realtime.response", "id": e.responseID, "status": status, "status_details": statusDetails, "output": append([]any(nil), e.output...), "conversation_id": nil, "output_modalities": []string{modality}, "max_output_tokens": "inf", "usage": usage, "metadata": nil}
}

func (e *RealtimeServerEncoder) messageItemSnapshot(status string, part map[string]any, completed bool) map[string]any {
	content := []any{}
	if completed {
		content = append(content, part)
	}
	return map[string]any{"id": e.activeItemID, "object": "realtime.item", "type": "message", "status": status, "role": "assistant", "content": content}
}

func (e *RealtimeServerEncoder) toolItemSnapshot(status, arguments string) map[string]any {
	return map[string]any{"id": e.activeItemID, "object": "realtime.item", "type": "function_call", "status": status, "call_id": e.activeCallID, "name": e.activeName, "arguments": arguments}
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
	e.model = ""
	e.text.Reset()
	e.arguments.Reset()
	e.usage = llm.Usage{}
	e.contentOpen = false
	e.stopped = false
	e.activeKind = ""
	e.activeIndex = 0
	e.activeItemID = ""
	e.activeCallID = ""
	e.activeName = ""
	e.output = nil
}
