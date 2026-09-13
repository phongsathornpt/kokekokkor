package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/sse"
)

type chatStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object,omitempty"`
	Model   string               `json:"model"`
	Choices []chatStreamChoice   `json:"choices"`
	Usage   *chatCompletionUsage `json:"usage,omitempty"`
}

type chatStreamChoice struct {
	Index        int             `json:"index"`
	Delta        chatStreamDelta `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

type chatStreamDelta struct {
	Role      string               `json:"role,omitempty"`
	Content   *string              `json:"content,omitempty"`
	ToolCalls []chatStreamToolCall `json:"tool_calls,omitempty"`
}

type chatStreamToolCall struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function chatStreamToolFunction `json:"function,omitempty"`
}

type chatStreamToolFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func ChatRequestStreams(data []byte) (bool, error) {
	var request struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return false, fmt.Errorf("decode OpenAI stream flag: %w", err)
	}
	return request.Stream, nil
}

func DecodeChatStream(r io.Reader, emit func(llm.StreamEvent) error) error {
	started := false
	done := false
	stopSeen := false
	var pendingStop llm.StopReason
	textBlock := -1
	nextBlock := 0
	toolBlocks := make(map[int]int)
	openBlocks := make(map[int]struct{})

	err := sse.Decode(r, func(event sse.Event) error {
		if string(event.Data) == "[DONE]" {
			done = true
			if pendingStop == "" || pendingStop == llm.StopReasonUnknown {
				return fmt.Errorf("OpenAI stream ended without a supported finish_reason")
			}
			if err := closeOpenBlocks(openBlocks, emit); err != nil {
				return err
			}
			stopSeen = true
			return emit(llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: pendingStop})
		}
		if len(event.Data) == 0 {
			return nil
		}

		var chunk chatStreamChunk
		if err := json.Unmarshal(event.Data, &chunk); err != nil {
			return fmt.Errorf("decode OpenAI stream chunk: %w", err)
		}
		if !started {
			started = true
			if err := emit(llm.StreamEvent{
				Type:       llm.StreamEventResponseStart,
				ResponseID: chunk.ID,
				Model:      chunk.Model,
			}); err != nil {
				return err
			}
		}

		if chunk.Usage != nil {
			usage := decodeChatStreamUsage(chunk.Usage)
			if err := emit(llm.StreamEvent{Type: llm.StreamEventUsage, Usage: &usage}); err != nil {
				return err
			}
		}

		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				return fmt.Errorf("OpenAI stream choice index %d is unsupported", choice.Index)
			}
			if choice.Delta.Content != nil {
				if textBlock < 0 {
					textBlock = nextBlock
					nextBlock++
					openBlocks[textBlock] = struct{}{}
					if err := emit(llm.StreamEvent{
						Type:  llm.StreamEventContentStart,
						Index: textBlock,
						Block: llm.TextBlock{},
					}); err != nil {
						return err
					}
				}
				if *choice.Delta.Content != "" {
					if err := emit(llm.StreamEvent{
						Type:      llm.StreamEventTextDelta,
						Index:     textBlock,
						TextDelta: *choice.Delta.Content,
					}); err != nil {
						return err
					}
				}
			}

			for _, call := range choice.Delta.ToolCalls {
				blockIndex, exists := toolBlocks[call.Index]
				if !exists {
					blockIndex = nextBlock
					nextBlock++
					toolBlocks[call.Index] = blockIndex
					openBlocks[blockIndex] = struct{}{}
					arguments := json.RawMessage(`{}`)
					if err := emit(llm.StreamEvent{
						Type:  llm.StreamEventToolCallStart,
						Index: blockIndex,
						Block: llm.ToolCallBlock{
							ID:        call.ID,
							Name:      call.Function.Name,
							Arguments: arguments,
						},
					}); err != nil {
						return err
					}
				}
				if call.Function.Arguments != "" {
					if err := emit(llm.StreamEvent{
						Type:  llm.StreamEventToolCallDelta,
						Index: blockIndex,
						ToolCallDelta: &llm.ToolCallDelta{
							ArgumentsDelta: call.Function.Arguments,
						},
					}); err != nil {
						return err
					}
				}
			}

			if choice.FinishReason != nil {
				pendingStop = decodeChatFinishReason(*choice.FinishReason)
				if pendingStop == llm.StopReasonUnknown {
					return fmt.Errorf("unsupported OpenAI finish_reason %q", *choice.FinishReason)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if started && !done && !stopSeen {
		return fmt.Errorf("OpenAI stream ended before [DONE]")
	}
	return nil
}

func closeOpenBlocks(open map[int]struct{}, emit func(llm.StreamEvent) error) error {
	indexes := make([]int, 0, len(open))
	for index := range open {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		if err := emit(llm.StreamEvent{Type: llm.StreamEventContentStop, Index: index}); err != nil {
			return err
		}
		delete(open, index)
	}
	return nil
}

func decodeChatStreamUsage(usage *chatCompletionUsage) llm.Usage {
	result := llm.Usage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}
	if usage.PromptTokensDetails != nil {
		result.CacheReadTokens = usage.PromptTokensDetails.CachedTokens
	}
	if usage.CompletionTokensDetails != nil {
		result.ReasoningTokens = usage.CompletionTokensDetails.ReasoningTokens
	}
	return result
}

type ChatStreamEncoder struct {
	w            io.Writer
	includeUsage bool
	id           string
	model        string
	started      bool
	stopped      bool
	latestUsage  *llm.Usage
	toolIndexes  map[int]int
	nextTool     int
}

func NewChatStreamEncoder(w io.Writer, includeUsage bool) *ChatStreamEncoder {
	return &ChatStreamEncoder{
		w:            w,
		includeUsage: includeUsage,
		toolIndexes:  make(map[int]int),
	}
}

func (e *ChatStreamEncoder) Encode(event llm.StreamEvent) error {
	switch event.Type {
	case llm.StreamEventResponseStart:
		e.id = event.ResponseID
		e.model = event.Model
		e.started = true
		return e.writeChoice(chatStreamDelta{Role: "assistant"}, nil)

	case llm.StreamEventTextDelta:
		text := event.TextDelta
		return e.writeChoice(chatStreamDelta{Content: &text}, nil)

	case llm.StreamEventToolCallStart:
		block, ok := event.Block.(llm.ToolCallBlock)
		if !ok {
			return fmt.Errorf("tool_call_start block is %T", event.Block)
		}
		toolIndex, ok := e.toolIndexes[event.Index]
		if !ok {
			toolIndex = e.nextTool
			e.nextTool++
			e.toolIndexes[event.Index] = toolIndex
		}
		return e.writeChoice(chatStreamDelta{ToolCalls: []chatStreamToolCall{{
			Index: toolIndex,
			ID:    block.ID,
			Type:  "function",
			Function: chatStreamToolFunction{
				Name: block.Name,
			},
		}}}, nil)

	case llm.StreamEventToolCallDelta:
		if event.ToolCallDelta == nil {
			return fmt.Errorf("tool_call_delta is missing delta payload")
		}
		toolIndex, ok := e.toolIndexes[event.Index]
		if !ok {
			return fmt.Errorf("tool_call_delta for unopened block %d", event.Index)
		}
		return e.writeChoice(chatStreamDelta{ToolCalls: []chatStreamToolCall{{
			Index: toolIndex,
			Function: chatStreamToolFunction{
				Arguments: event.ToolCallDelta.ArgumentsDelta,
			},
		}}}, nil)

	case llm.StreamEventUsage:
		if event.Usage != nil {
			copy := *event.Usage
			e.latestUsage = &copy
		}
		return nil

	case llm.StreamEventResponseStop:
		finish, err := encodeChatFinishReason(event.StopReason)
		if err != nil {
			return err
		}
		if err := e.writeChoice(chatStreamDelta{}, &finish); err != nil {
			return err
		}
		if e.includeUsage && e.latestUsage != nil {
			if err := e.writeUsage(*e.latestUsage); err != nil {
				return err
			}
		}
		e.stopped = true
		return sse.Write(e.w, sse.Event{Data: []byte("[DONE]")})

	case llm.StreamEventContentStart, llm.StreamEventContentStop:
		return nil
	case llm.StreamEventReasoningDelta:
		return fmt.Errorf("reasoning stream events are not supported by Chat Completions translation")
	case llm.StreamEventError:
		return fmt.Errorf("upstream stream error cannot be represented as a Chat Completions chunk")
	default:
		return fmt.Errorf("unsupported canonical stream event %q", event.Type)
	}
}

func (e *ChatStreamEncoder) writeChoice(delta chatStreamDelta, finish *string) error {
	chunk := chatStreamChunk{
		ID:     e.id,
		Object: "chat.completion.chunk",
		Model:  e.model,
		Choices: []chatStreamChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: finish,
		}},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	return sse.Write(e.w, sse.Event{Data: data})
}

func (e *ChatStreamEncoder) writeUsage(usage llm.Usage) error {
	wire := &chatCompletionUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.InputTokens + usage.OutputTokens,
	}
	if usage.CacheReadTokens != 0 {
		wire.PromptTokensDetails = &promptTokenDetails{CachedTokens: usage.CacheReadTokens}
	}
	if usage.ReasoningTokens != 0 {
		wire.CompletionTokensDetails = &completionTokenDetails{ReasoningTokens: usage.ReasoningTokens}
	}
	chunk := chatStreamChunk{
		ID:      e.id,
		Object:  "chat.completion.chunk",
		Model:   e.model,
		Choices: []chatStreamChoice{},
		Usage:   wire,
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	return sse.Write(e.w, sse.Event{Data: data})
}
