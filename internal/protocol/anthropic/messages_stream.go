package anthropic

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/sse"
)

func MessagesRequestStreams(data []byte) (bool, error) {
	var request struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return false, fmt.Errorf("decode Anthropic stream flag: %w", err)
	}
	return request.Stream, nil
}

type streamUsage struct {
	InputTokens              *int64 `json:"input_tokens,omitempty"`
	OutputTokens             *int64 `json:"output_tokens,omitempty"`
	CacheReadInputTokens     *int64 `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens *int64 `json:"cache_creation_input_tokens,omitempty"`
}

func DecodeMessagesStream(r io.Reader, emit func(llm.StreamEvent) error) error {
	started := false
	stopped := false
	var usage llm.Usage
	thinkingBlocks := make(map[int]bool)
	thinkingStarted := make(map[int]bool)
	var pendingStop llm.StopReason
	var pendingSequence string

	err := sse.Decode(r, func(event sse.Event) error {
		if len(event.Data) == 0 {
			return nil
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(event.Data, &envelope); err != nil {
			return fmt.Errorf("decode Anthropic stream event: %w", err)
		}

		switch envelope.Type {
		case "ping":
			return nil

		case "message_start":
			var wire struct {
				Message struct {
					ID    string      `json:"id"`
					Model string      `json:"model"`
					Usage streamUsage `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &wire); err != nil {
				return err
			}
			started = true
			if err := emit(llm.StreamEvent{
				Type:       llm.StreamEventResponseStart,
				ResponseID: wire.Message.ID,
				Model:      wire.Message.Model,
			}); err != nil {
				return err
			}
			mergeStreamUsage(&usage, wire.Message.Usage)
			copyUsage := usage
			return emit(llm.StreamEvent{Type: llm.StreamEventUsage, Usage: &copyUsage})

		case "content_block_start":
			var wire struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type     string          `json:"type"`
					Text     string          `json:"text,omitempty"`
					Thinking string          `json:"thinking,omitempty"`
					ID       string          `json:"id,omitempty"`
					Name     string          `json:"name,omitempty"`
					Input    json.RawMessage `json:"input,omitempty"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal(event.Data, &wire); err != nil {
				return err
			}
			switch wire.ContentBlock.Type {
			case "text":
				if err := emit(llm.StreamEvent{
					Type:  llm.StreamEventContentStart,
					Index: wire.Index,
					Block: llm.TextBlock{Text: wire.ContentBlock.Text},
				}); err != nil {
					return err
				}
				if wire.ContentBlock.Text != "" {
					return emit(llm.StreamEvent{Type: llm.StreamEventTextDelta, Index: wire.Index, TextDelta: wire.ContentBlock.Text})
				}
				return nil
			case "tool_use":
				arguments := wire.ContentBlock.Input
				if len(arguments) == 0 || string(arguments) == "null" {
					arguments = json.RawMessage(`{}`)
				}
				return emit(llm.StreamEvent{
					Type:  llm.StreamEventToolCallStart,
					Index: wire.Index,
					Block: llm.ToolCallBlock{
						ID:        wire.ContentBlock.ID,
						Name:      wire.ContentBlock.Name,
						Arguments: arguments,
					},
				})
			case "thinking":
				thinkingBlocks[wire.Index] = true
				if wire.ContentBlock.Thinking == "" {
					return nil
				}
				thinkingStarted[wire.Index] = true
				if err := emit(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: wire.Index, Block: llm.ReasoningBlock{}}); err != nil {
					return err
				}
				return emit(llm.StreamEvent{Type: llm.StreamEventReasoningDelta, Index: wire.Index, ReasoningDelta: wire.ContentBlock.Thinking})
			case "redacted_thinking":
				thinkingBlocks[wire.Index] = true
				return nil
			default:
				return fmt.Errorf("unsupported Anthropic stream content block %q", wire.ContentBlock.Type)
			}

		case "content_block_delta":
			var wire struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text,omitempty"`
					PartialJSON string `json:"partial_json,omitempty"`
					Thinking    string `json:"thinking,omitempty"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(event.Data, &wire); err != nil {
				return err
			}
			switch wire.Delta.Type {
			case "text_delta":
				return emit(llm.StreamEvent{Type: llm.StreamEventTextDelta, Index: wire.Index, TextDelta: wire.Delta.Text})
			case "input_json_delta":
				return emit(llm.StreamEvent{
					Type:  llm.StreamEventToolCallDelta,
					Index: wire.Index,
					ToolCallDelta: &llm.ToolCallDelta{
						ArgumentsDelta: wire.Delta.PartialJSON,
					},
				})
			case "thinking_delta":
				if !thinkingBlocks[wire.Index] {
					return fmt.Errorf("Anthropic thinking delta for unopened block %d", wire.Index)
				}
				if !thinkingStarted[wire.Index] {
					thinkingStarted[wire.Index] = true
					if err := emit(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: wire.Index, Block: llm.ReasoningBlock{}}); err != nil {
						return err
					}
				}
				if wire.Delta.Thinking == "" {
					return nil
				}
				return emit(llm.StreamEvent{Type: llm.StreamEventReasoningDelta, Index: wire.Index, ReasoningDelta: wire.Delta.Thinking})
			case "signature_delta":
				if !thinkingBlocks[wire.Index] {
					return fmt.Errorf("Anthropic signature delta for unopened block %d", wire.Index)
				}
				return nil
			default:
				return fmt.Errorf("unsupported Anthropic stream delta %q", wire.Delta.Type)
			}

		case "content_block_stop":
			var wire struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal(event.Data, &wire); err != nil {
				return err
			}
			if thinkingBlocks[wire.Index] {
				delete(thinkingBlocks, wire.Index)
				started := thinkingStarted[wire.Index]
				delete(thinkingStarted, wire.Index)
				if !started {
					return nil
				}
			}
			if thinkingBlocks[wire.Index] {
				delete(thinkingBlocks, wire.Index)
				started := thinkingStarted[wire.Index]
				delete(thinkingStarted, wire.Index)
				if !started {
					return nil
				}
			}
			return emit(llm.StreamEvent{Type: llm.StreamEventContentStop, Index: wire.Index})

		case "message_delta":
			var wire struct {
				Delta struct {
					StopReason   string  `json:"stop_reason"`
					StopSequence *string `json:"stop_sequence"`
				} `json:"delta"`
				Usage streamUsage `json:"usage"`
			}
			if err := json.Unmarshal(event.Data, &wire); err != nil {
				return err
			}
			if wire.Delta.StopReason != "" {
				pendingStop = decodeAnthropicStopReason(wire.Delta.StopReason)
				if pendingStop == llm.StopReasonUnknown {
					return fmt.Errorf("unsupported Anthropic stop_reason %q", wire.Delta.StopReason)
				}
			}
			if wire.Delta.StopSequence != nil {
				pendingSequence = *wire.Delta.StopSequence
			}
			mergeStreamUsage(&usage, wire.Usage)
			copyUsage := usage
			return emit(llm.StreamEvent{Type: llm.StreamEventUsage, Usage: &copyUsage})

		case "message_stop":
			if pendingStop == "" || pendingStop == llm.StopReasonUnknown {
				return fmt.Errorf("Anthropic stream ended without a supported stop_reason")
			}
			stopped = true
			return emit(llm.StreamEvent{
				Type:         llm.StreamEventResponseStop,
				StopReason:   pendingStop,
				StopSequence: pendingSequence,
			})

		case "error":
			var wire struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(event.Data, &wire); err != nil {
				return err
			}
			return fmt.Errorf("Anthropic stream error %s: %s", wire.Error.Type, wire.Error.Message)

		default:
			return fmt.Errorf("unsupported Anthropic stream event %q", envelope.Type)
		}
	})
	if err != nil {
		return err
	}
	if started && !stopped {
		return fmt.Errorf("Anthropic stream ended before message_stop")
	}
	return nil
}

func mergeStreamUsage(dst *llm.Usage, wire streamUsage) {
	if wire.InputTokens != nil {
		dst.InputTokens = *wire.InputTokens
	}
	if wire.OutputTokens != nil {
		dst.OutputTokens = *wire.OutputTokens
	}
	if wire.CacheReadInputTokens != nil {
		dst.CacheReadTokens = *wire.CacheReadInputTokens
	}
	if wire.CacheCreationInputTokens != nil {
		dst.CacheWriteTokens = *wire.CacheCreationInputTokens
	}
}

type MessagesStreamEncoder struct {
	w           io.Writer
	id          string
	model       string
	started     bool
	latestUsage llm.Usage
}

func NewMessagesStreamEncoder(w io.Writer) *MessagesStreamEncoder {
	return &MessagesStreamEncoder{w: w}
}

func (e *MessagesStreamEncoder) Encode(event llm.StreamEvent) error {
	switch event.Type {
	case llm.StreamEventResponseStart:
		e.id = event.ResponseID
		e.model = event.Model
		e.started = true
		return e.write("message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":            e.id,
				"type":          "message",
				"role":          "assistant",
				"content":       []any{},
				"model":         e.model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		})

	case llm.StreamEventContentStart:
		block, ok := event.Block.(llm.TextBlock)
		if !ok {
			return fmt.Errorf("content_start block is %T", event.Block)
		}
		return e.write("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": event.Index,
			"content_block": map[string]any{
				"type": "text",
				"text": block.Text,
			},
		})

	case llm.StreamEventToolCallStart:
		block, ok := event.Block.(llm.ToolCallBlock)
		if !ok {
			return fmt.Errorf("tool_call_start block is %T", event.Block)
		}
		return e.write("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": event.Index,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    block.ID,
				"name":  block.Name,
				"input": map[string]any{},
			},
		})

	case llm.StreamEventTextDelta:
		return e.write("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": event.Index,
			"delta": map[string]any{
				"type": "text_delta",
				"text": event.TextDelta,
			},
		})

	case llm.StreamEventToolCallDelta:
		if event.ToolCallDelta == nil {
			return fmt.Errorf("tool_call_delta is missing delta payload")
		}
		return e.write("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": event.Index,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": event.ToolCallDelta.ArgumentsDelta,
			},
		})

	case llm.StreamEventContentStop:
		return e.write("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": event.Index,
		})

	case llm.StreamEventUsage:
		if event.Usage != nil {
			e.latestUsage = *event.Usage
		}
		return nil

	case llm.StreamEventResponseStop:
		stopReason, err := encodeAnthropicStopReason(event.StopReason)
		if err != nil {
			return err
		}
		var stopSequence any
		if event.StopSequence != "" {
			stopSequence = event.StopSequence
		}
		if err := e.write("message_delta", map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   stopReason,
				"stop_sequence": stopSequence,
			},
			"usage": map[string]any{
				"input_tokens":                e.latestUsage.InputTokens,
				"output_tokens":               e.latestUsage.OutputTokens,
				"cache_read_input_tokens":     e.latestUsage.CacheReadTokens,
				"cache_creation_input_tokens": e.latestUsage.CacheWriteTokens,
			},
		}); err != nil {
			return err
		}
		return e.write("message_stop", map[string]any{"type": "message_stop"})

	case llm.StreamEventReasoningDelta:
		return fmt.Errorf("reasoning stream events are not supported by Anthropic translation")
	case llm.StreamEventError:
		return fmt.Errorf("canonical stream error cannot be represented after translation")
	default:
		return fmt.Errorf("unsupported canonical stream event %q", event.Type)
	}
}

func (e *MessagesStreamEncoder) write(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return sse.Write(e.w, sse.Event{Name: name, Data: data})
}
