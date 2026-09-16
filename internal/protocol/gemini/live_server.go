package gemini

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

type LiveServerContent struct {
	Text               []string
	GenerationComplete bool
	TurnComplete       bool
	Interrupted        bool
	Metadata           map[string]json.RawMessage
}

type LiveServerMessage struct {
	SetupComplete           bool
	ServerContent           *LiveServerContent
	Usage                   *llm.Usage
	ToolCall                json.RawMessage
	ToolCallCancellation    json.RawMessage
	GoAway                  json.RawMessage
	SessionResumptionUpdate json.RawMessage
	Metadata                map[string]json.RawMessage
}

type liveServerEnvelope struct {
	SetupComplete           json.RawMessage `json:"setupComplete"`
	ServerContent           json.RawMessage `json:"serverContent"`
	UsageMetadata           json.RawMessage `json:"usageMetadata"`
	ToolCall                json.RawMessage `json:"toolCall"`
	ToolCallCancellation    json.RawMessage `json:"toolCallCancellation"`
	GoAway                  json.RawMessage `json:"goAway"`
	SessionResumptionUpdate json.RawMessage `json:"sessionResumptionUpdate"`
}

type liveServerContentEnvelope struct {
	ModelTurn          liveContent `json:"modelTurn"`
	GenerationComplete bool        `json:"generationComplete"`
	TurnComplete       bool        `json:"turnComplete"`
	Interrupted        bool        `json:"interrupted"`
}

type liveContent struct {
	Parts []livePart `json:"parts"`
}

type livePart struct {
	Text *string `json:"text"`
}

type liveUsage struct {
	PromptTokenCount        int64 `json:"promptTokenCount"`
	ResponseTokenCount      int64 `json:"responseTokenCount"`
	ThoughtsTokenCount      int64 `json:"thoughtsTokenCount"`
	CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
}

func DecodeLiveServerMessage(data []byte) (LiveServerMessage, error) {
	var wire liveServerEnvelope
	if err := json.Unmarshal(data, &wire); err != nil {
		return LiveServerMessage{}, fmt.Errorf("decode Gemini Live server message: %w", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return LiveServerMessage{}, fmt.Errorf("decode Gemini Live server message fields: %w", err)
	}

	message := LiveServerMessage{}
	messageTypeCount := 0
	if len(wire.SetupComplete) != 0 {
		message.SetupComplete = true
		messageTypeCount++
		delete(fields, "setupComplete")
	}
	if len(wire.ServerContent) != 0 {
		content, err := decodeLiveServerContent(wire.ServerContent)
		if err != nil {
			return LiveServerMessage{}, err
		}
		message.ServerContent = &content
		messageTypeCount++
		delete(fields, "serverContent")
	}
	if len(wire.ToolCall) != 0 {
		message.ToolCall = cloneRawMessage(wire.ToolCall)
		messageTypeCount++
		delete(fields, "toolCall")
	}
	if len(wire.ToolCallCancellation) != 0 {
		message.ToolCallCancellation = cloneRawMessage(wire.ToolCallCancellation)
		messageTypeCount++
		delete(fields, "toolCallCancellation")
	}
	if len(wire.GoAway) != 0 {
		message.GoAway = cloneRawMessage(wire.GoAway)
		messageTypeCount++
		delete(fields, "goAway")
	}
	if len(wire.SessionResumptionUpdate) != 0 {
		message.SessionResumptionUpdate = cloneRawMessage(wire.SessionResumptionUpdate)
		messageTypeCount++
		delete(fields, "sessionResumptionUpdate")
	}
	if messageTypeCount > 1 {
		return LiveServerMessage{}, fmt.Errorf("decode Gemini Live server message: multiple messageType fields")
	}

	if len(wire.UsageMetadata) != 0 {
		usage, err := decodeLiveUsage(wire.UsageMetadata)
		if err != nil {
			return LiveServerMessage{}, err
		}
		message.Usage = &usage
		delete(fields, "usageMetadata")
	}
	if len(fields) != 0 {
		message.Metadata = fields
	}
	return message, nil
}

func decodeLiveServerContent(data json.RawMessage) (LiveServerContent, error) {
	var wire liveServerContentEnvelope
	if err := json.Unmarshal(data, &wire); err != nil {
		return LiveServerContent{}, fmt.Errorf("decode Gemini Live server content: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return LiveServerContent{}, fmt.Errorf("decode Gemini Live server content fields: %w", err)
	}
	delete(fields, "modelTurn")
	delete(fields, "generationComplete")
	delete(fields, "turnComplete")
	delete(fields, "interrupted")

	content := LiveServerContent{
		GenerationComplete: wire.GenerationComplete,
		TurnComplete:       wire.TurnComplete,
		Interrupted:        wire.Interrupted,
	}
	for _, part := range wire.ModelTurn.Parts {
		if part.Text != nil {
			content.Text = append(content.Text, *part.Text)
		}
	}
	if len(fields) != 0 {
		content.Metadata = fields
	}
	return content, nil
}

func decodeLiveUsage(data json.RawMessage) (llm.Usage, error) {
	var wire liveUsage
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Usage{}, fmt.Errorf("decode Gemini Live usage: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return llm.Usage{}, fmt.Errorf("decode Gemini Live usage fields: %w", err)
	}
	delete(fields, "promptTokenCount")
	delete(fields, "responseTokenCount")
	delete(fields, "thoughtsTokenCount")
	delete(fields, "cachedContentTokenCount")
	usage := llm.Usage{
		InputTokens:     wire.PromptTokenCount,
		OutputTokens:    wire.ResponseTokenCount,
		ReasoningTokens: wire.ThoughtsTokenCount,
		CacheReadTokens: wire.CachedContentTokenCount,
	}
	if len(fields) != 0 {
		usage.Metadata = fields
	}
	return usage, nil
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}
