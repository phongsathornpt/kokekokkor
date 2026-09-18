package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type responseStateStreamTool struct {
	id        string
	name      string
	initial   json.RawMessage
	arguments strings.Builder
}

type responseStateStreamAccumulator struct {
	responseID  string
	text        map[int]*strings.Builder
	tools       map[int]*responseStateStreamTool
	nonportable bool
}

func newResponseStateStreamAccumulator() *responseStateStreamAccumulator {
	return &responseStateStreamAccumulator{
		text:  make(map[int]*strings.Builder),
		tools: make(map[int]*responseStateStreamTool),
	}
}

func (a *responseStateStreamAccumulator) Observe(event llm.StreamEvent) error {
	switch event.Type {
	case llm.StreamEventResponseStart:
		a.responseID = event.ResponseID
	case llm.StreamEventContentStart:
		switch block := event.Block.(type) {
		case llm.TextBlock:
			var builder strings.Builder
			builder.WriteString(block.Text)
			a.text[event.Index] = &builder
		case llm.ReasoningBlock:
			a.nonportable = true
		default:
			a.nonportable = true
		}
	case llm.StreamEventTextDelta:
		builder := a.text[event.Index]
		if builder == nil {
			return fmt.Errorf("response state text delta for unopened block %d", event.Index)
		}
		builder.WriteString(event.TextDelta)
	case llm.StreamEventToolCallStart:
		block, ok := event.Block.(llm.ToolCallBlock)
		if !ok {
			return fmt.Errorf("response state tool start contains %T", event.Block)
		}
		a.tools[event.Index] = &responseStateStreamTool{
			id:      block.ID,
			name:    block.Name,
			initial: append(json.RawMessage(nil), block.Arguments...),
		}
	case llm.StreamEventToolCallDelta:
		tool := a.tools[event.Index]
		if tool == nil || event.ToolCallDelta == nil {
			return fmt.Errorf("response state tool delta for unopened block %d", event.Index)
		}
		tool.arguments.WriteString(event.ToolCallDelta.ArgumentsDelta)
	case llm.StreamEventReasoningDelta:
		a.nonportable = true
	}
	return nil
}

func (a *responseStateStreamAccumulator) Persist(ctx context.Context, runtime *Runtime, plan responseStatePlan) error {
	if plan.state == nil || !plan.state.Store {
		return nil
	}
	if a.responseID == "" {
		return fmt.Errorf("streamed response state is missing response id")
	}
	indexSet := make(map[int]struct{}, len(a.text)+len(a.tools))
	for index := range a.text {
		indexSet[index] = struct{}{}
	}
	for index := range a.tools {
		indexSet[index] = struct{}{}
	}
	indexes := make([]int, 0, len(indexSet))
	for index := range indexSet {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	content := make([]llm.ContentBlock, 0, len(indexes))
	for _, index := range indexes {
		if text := a.text[index]; text != nil {
			content = append(content, llm.TextBlock{Text: text.String()})
			continue
		}
		tool := a.tools[index]
		arguments := json.RawMessage(tool.arguments.String())
		if len(arguments) == 0 {
			arguments = append(json.RawMessage(nil), tool.initial...)
		}
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}
		if !json.Valid(arguments) {
			return fmt.Errorf("streamed response state tool %q has invalid arguments", tool.name)
		}
		content = append(content, llm.ToolCallBlock{
			ID:        tool.id,
			Name:      tool.name,
			Arguments: arguments,
		})
	}
	if a.nonportable {
		content = append(content, llm.ReasoningBlock{})
	}
	return runtime.persistResponsesState(ctx, plan, llm.Response{
		ID:      a.responseID,
		Content: content,
	})
}
