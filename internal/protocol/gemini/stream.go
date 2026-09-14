package gemini

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	"github.com/phongsathornpt/kokekokkor/internal/protocol/sse"
)

func StreamGenerateContentPath(model string) (string, error) {
	model = strings.TrimSpace(strings.TrimPrefix(model, "models/"))
	if model == "" {
		return "", fmt.Errorf("Gemini model must not be empty")
	}
	return "/v1beta/models/" + url.PathEscape(model) + ":streamGenerateContent", nil
}

type geminiDecodeBlock struct {
	index     int
	kind      string
	toolID    string
	toolName  string
	arguments string
}

type geminiStreamDecoder struct {
	emit       func(llm.StreamEvent) error
	started    bool
	stopped    bool
	sawTool    bool
	nextBlock  int
	active     *geminiDecodeBlock
	responseID string
	model      string
}

func DecodeGenerateContentStream(r io.Reader, emit func(llm.StreamEvent) error) error {
	decoder := &geminiStreamDecoder{emit: emit}
	if err := sse.Decode(r, decoder.decodeEvent); err != nil {
		return err
	}
	if decoder.started && !decoder.stopped {
		return fmt.Errorf("Gemini stream ended without finishReason")
	}
	return nil
}

func (d *geminiStreamDecoder) decodeEvent(event sse.Event) error {
	if len(event.Data) == 0 || string(event.Data) == "[DONE]" {
		return nil
	}
	var chunk generateContentResponse
	if err := json.Unmarshal(event.Data, &chunk); err != nil {
		return fmt.Errorf("decode Gemini stream chunk: %w", err)
	}
	if !d.started {
		d.started = true
		d.responseID = chunk.ResponseID
		d.model = chunk.ModelVersion
		if err := d.emit(llm.StreamEvent{
			Type:       llm.StreamEventResponseStart,
			ResponseID: d.responseID,
			Model:      d.model,
		}); err != nil {
			return err
		}
	}
	if chunk.UsageMetadata != nil {
		usage := llm.Usage{
			InputTokens:     chunk.UsageMetadata.PromptTokenCount,
			OutputTokens:    chunk.UsageMetadata.CandidatesTokenCount,
			CacheReadTokens: chunk.UsageMetadata.CachedContentTokenCount,
			ReasoningTokens: chunk.UsageMetadata.ThoughtsTokenCount,
		}
		if err := d.emit(llm.StreamEvent{Type: llm.StreamEventUsage, Usage: &usage}); err != nil {
			return err
		}
	}
	if len(chunk.Candidates) > 1 {
		return fmt.Errorf("Gemini stream chunk has %d candidates; translation requires exactly one", len(chunk.Candidates))
	}
	if len(chunk.Candidates) == 0 {
		return nil
	}
	candidate := chunk.Candidates[0]
	for _, part := range candidate.Content.Parts {
		if err := d.decodePart(part); err != nil {
			return err
		}
	}
	if candidate.FinishReason == "" {
		return nil
	}
	stop := decodeGeminiFinishReason(candidate.FinishReason, nil)
	if d.sawTool && stop == llm.StopReasonEndTurn {
		stop = llm.StopReasonToolUse
	}
	if stop == llm.StopReasonUnknown {
		return fmt.Errorf("unsupported Gemini finishReason %q", candidate.FinishReason)
	}
	if err := d.closeActive(); err != nil {
		return err
	}
	d.stopped = true
	return d.emit(llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: stop})
}

func (d *geminiStreamDecoder) decodePart(part geminiPart) error {
	if part.Thought {
		if part.Text == nil || *part.Text == "" {
			return nil
		}
		if err := d.ensureReasoning(); err != nil {
			return err
		}
		return d.emit(llm.StreamEvent{Type: llm.StreamEventReasoningDelta, Index: d.active.index, ReasoningDelta: *part.Text})
	}
	if part.ThoughtSignature != "" && part.Text != nil && *part.Text == "" && part.FunctionCall == nil && part.InlineData == nil && part.FileData == nil && part.FunctionResponse == nil {
		return nil
	}
	if part.Text != nil {
		if err := d.ensureText(); err != nil {
			return err
		}
		if *part.Text == "" {
			return nil
		}
		return d.emit(llm.StreamEvent{
			Type:      llm.StreamEventTextDelta,
			Index:     d.active.index,
			TextDelta: *part.Text,
		})
	}
	if part.FunctionCall != nil {
		return d.decodeFunctionCall(*part.FunctionCall)
	}
	if part.InlineData != nil || part.FileData != nil || part.FunctionResponse != nil {
		return fmt.Errorf("Gemini streamed output part cannot be represented by the current canonical stream")
	}
	return fmt.Errorf("Gemini stream contained an empty or unsupported part")
}

func (d *geminiStreamDecoder) ensureText() error {
	if d.active != nil && d.active.kind == "text" {
		return nil
	}
	if err := d.closeActive(); err != nil {
		return err
	}
	block := &geminiDecodeBlock{index: d.nextBlock, kind: "text"}
	d.nextBlock++
	d.active = block
	return d.emit(llm.StreamEvent{
		Type:  llm.StreamEventContentStart,
		Index: block.index,
		Block: llm.TextBlock{},
	})
}

func (d *geminiStreamDecoder) ensureReasoning() error {
	if d.active != nil && d.active.kind == "reasoning" {
		return nil
	}
	if err := d.closeActive(); err != nil {
		return err
	}
	block := &geminiDecodeBlock{index: d.nextBlock, kind: "reasoning"}
	d.nextBlock++
	d.active = block
	return d.emit(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: block.index, Block: llm.ReasoningBlock{}})
}

func (d *geminiStreamDecoder) ensureReasoning() error {
	if d.active != nil && d.active.kind == "reasoning" {
		return nil
	}
	if err := d.closeActive(); err != nil {
		return err
	}
	block := &geminiDecodeBlock{index: d.nextBlock, kind: "reasoning"}
	d.nextBlock++
	d.active = block
	return d.emit(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: block.index, Block: llm.ReasoningBlock{}})
}

func (d *geminiStreamDecoder) ensureReasoning() error {
	if d.active != nil && d.active.kind == "reasoning" {
		return nil
	}
	if err := d.closeActive(); err != nil {
		return err
	}
	block := &geminiDecodeBlock{index: d.nextBlock, kind: "reasoning"}
	d.nextBlock++
	d.active = block
	return d.emit(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: block.index, Block: llm.ReasoningBlock{}})
}

func (d *geminiStreamDecoder) ensureReasoning() error {
	if d.active != nil && d.active.kind == "reasoning" {
		return nil
	}
	if err := d.closeActive(); err != nil {
		return err
	}
	block := &geminiDecodeBlock{index: d.nextBlock, kind: "reasoning"}
	d.nextBlock++
	d.active = block
	return d.emit(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: block.index, Block: llm.ReasoningBlock{}})
}

func (d *geminiStreamDecoder) ensureReasoning() error {
	if d.active != nil && d.active.kind == "reasoning" {
		return nil
	}
	if err := d.closeActive(); err != nil {
		return err
	}
	block := &geminiDecodeBlock{index: d.nextBlock, kind: "reasoning"}
	d.nextBlock++
	d.active = block
	return d.emit(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: block.index, Block: llm.ReasoningBlock{}})
}

func (d *geminiStreamDecoder) decodeFunctionCall(call geminiFunctionCall) error {
	id := call.ID
	if id == "" && d.active != nil && d.active.kind == "tool" && d.active.toolName == call.Name {
		id = d.active.toolID
	}
	if id == "" {
		id = fmt.Sprintf("gemini_call_%d", d.nextBlock)
	}
	if d.active == nil || d.active.kind != "tool" || d.active.toolID != id || d.active.toolName != call.Name {
		if err := d.closeActive(); err != nil {
			return err
		}
		block := &geminiDecodeBlock{index: d.nextBlock, kind: "tool", toolID: id, toolName: call.Name}
		d.nextBlock++
		d.active = block
		d.sawTool = true
		if err := d.emit(llm.StreamEvent{
			Type:  llm.StreamEventToolCallStart,
			Index: block.index,
			Block: llm.ToolCallBlock{ID: id, Name: call.Name, Arguments: json.RawMessage(`{}`)},
		}); err != nil {
			return err
		}
	}
	args := call.Args
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if !isJSONObject(args) {
		return fmt.Errorf("Gemini function call %q args must be a JSON object", call.Name)
	}
	encoded, err := compactJSON(args)
	if err != nil {
		return err
	}
	previous := d.active.arguments
	if encoded == previous {
		return nil
	}
	if previous != "" && !strings.HasPrefix(encoded, previous) {
		return fmt.Errorf("Gemini function call %q changed non-incrementally across stream chunks", call.Name)
	}
	delta := strings.TrimPrefix(encoded, previous)
	d.active.arguments = encoded
	if delta == "" {
		return nil
	}
	return d.emit(llm.StreamEvent{
		Type:  llm.StreamEventToolCallDelta,
		Index: d.active.index,
		ToolCallDelta: &llm.ToolCallDelta{
			ID:             id,
			Name:           call.Name,
			ArgumentsDelta: delta,
		},
	})
}

func (d *geminiStreamDecoder) closeActive() error {
	if d.active == nil {
		return nil
	}
	index := d.active.index
	d.active = nil
	return d.emit(llm.StreamEvent{Type: llm.StreamEventContentStop, Index: index})
}

type geminiEncodeBlock struct {
	index     int
	toolID    string
	toolName  string
	arguments strings.Builder
}

type GenerateContentStreamEncoder struct {
	w           io.Writer
	responseID  string
	model       string
	started     bool
	stopped     bool
	latestUsage llm.Usage
	tools       map[int]*geminiEncodeBlock
}

func NewGenerateContentStreamEncoder(w io.Writer) *GenerateContentStreamEncoder {
	return &GenerateContentStreamEncoder{w: w, tools: make(map[int]*geminiEncodeBlock)}
}

func (e *GenerateContentStreamEncoder) Encode(event llm.StreamEvent) error {
	switch event.Type {
	case llm.StreamEventResponseStart:
		if e.started {
			return fmt.Errorf("Gemini stream received duplicate response start")
		}
		e.started = true
		e.responseID = event.ResponseID
		e.model = event.Model
		return nil
	case llm.StreamEventContentStart:
		return nil
	case llm.StreamEventTextDelta:
		return e.writeChunk([]geminiPart{{Text: stringPointer(event.TextDelta)}}, "", nil)
	case llm.StreamEventToolCallStart:
		block, ok := event.Block.(llm.ToolCallBlock)
		if !ok {
			return fmt.Errorf("tool_call_start block is %T", event.Block)
		}
		e.tools[event.Index] = &geminiEncodeBlock{index: event.Index, toolID: block.ID, toolName: block.Name}
		return nil
	case llm.StreamEventToolCallDelta:
		state, ok := e.tools[event.Index]
		if !ok {
			return fmt.Errorf("tool_call_delta for unopened block %d", event.Index)
		}
		if event.ToolCallDelta == nil {
			return fmt.Errorf("tool_call_delta is missing delta payload")
		}
		state.arguments.WriteString(event.ToolCallDelta.ArgumentsDelta)
		return nil
	case llm.StreamEventContentStop:
		state, ok := e.tools[event.Index]
		if !ok {
			return nil
		}
		delete(e.tools, event.Index)
		arguments := state.arguments.String()
		if arguments == "" {
			arguments = "{}"
		}
		raw := json.RawMessage(arguments)
		if !isJSONObject(raw) {
			return fmt.Errorf("tool call %q streamed arguments are not a JSON object", state.toolName)
		}
		return e.writeChunk([]geminiPart{{FunctionCall: &geminiFunctionCall{
			ID: state.toolID, Name: state.toolName, Args: raw,
		}}}, "", nil)
	case llm.StreamEventUsage:
		if event.Usage != nil {
			e.latestUsage = *event.Usage
		}
		return nil
	case llm.StreamEventResponseStop:
		if len(e.tools) != 0 {
			indexes := make([]int, 0, len(e.tools))
			for index := range e.tools {
				indexes = append(indexes, index)
			}
			sort.Ints(indexes)
			return fmt.Errorf("Gemini stream stopped with open tool block %d", indexes[0])
		}
		finish, err := encodeGeminiFinishReason(event.StopReason)
		if err != nil {
			return err
		}
		e.stopped = true
		usage := geminiUsageMetadata{
			PromptTokenCount:        e.latestUsage.InputTokens,
			CandidatesTokenCount:    e.latestUsage.OutputTokens,
			CachedContentTokenCount: e.latestUsage.CacheReadTokens,
			ThoughtsTokenCount:      e.latestUsage.ReasoningTokens,
			TotalTokenCount:         e.latestUsage.InputTokens + e.latestUsage.OutputTokens,
		}
		return e.writeChunk(nil, finish, &usage)
	case llm.StreamEventReasoningDelta:
		return fmt.Errorf("reasoning stream events are not supported by Gemini generateContent translation")
	case llm.StreamEventError:
		return fmt.Errorf("upstream stream errors cannot be represented as Gemini generateContent chunks")
	default:
		return fmt.Errorf("unsupported canonical stream event %q", event.Type)
	}
}

func (e *GenerateContentStreamEncoder) writeChunk(parts []geminiPart, finish string, usage *geminiUsageMetadata) error {
	chunk := generateContentResponse{
		Candidates: []geminiCandidate{{
			Content:      geminiContent{Role: "model", Parts: parts},
			FinishReason: finish,
		}},
		UsageMetadata: usage,
		ModelVersion:  e.model,
		ResponseID:    e.responseID,
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	return sse.Write(e.w, sse.Event{Data: data})
}

func stringPointer(value string) *string { return &value }
