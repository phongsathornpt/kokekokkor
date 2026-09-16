package gemini

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

const geminiLiveOutputAudioMediaType = "audio/pcm;rate=24000"

// LiveStreamDecoder converts the portable text/function/audio subset of Gemini
// Live server messages into canonical stream events.
type LiveStreamDecoder struct {
	model        string
	responseOpen bool
	contentOpen  bool
	contentKind  string
}

type liveToolCallEnvelope struct {
	FunctionCalls []struct {
		ID   string          `json:"id"`
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"functionCalls"`
}

func NewLiveStreamDecoder(model string) *LiveStreamDecoder {
	return &LiveStreamDecoder{model: model}
}

func (d *LiveStreamDecoder) Decode(message LiveServerMessage) ([]llm.StreamEvent, error) {
	if len(message.Metadata) != 0 {
		return nil, errors.New("Gemini Live server message contains provider-specific metadata")
	}
	if len(message.ToolCallCancellation) != 0 {
		return nil, errors.New("Gemini Live tool-call cancellation has no portable OpenAI Realtime mapping")
	}
	if len(message.GoAway) != 0 || len(message.SessionResumptionUpdate) != 0 || message.SetupComplete {
		if message.ServerContent == nil && message.Usage == nil && len(message.ToolCall) == 0 {
			return nil, nil
		}
	}
	if message.ServerContent != nil && len(message.ServerContent.Metadata) != 0 {
		return nil, errors.New("Gemini Live server content contains provider-specific metadata")
	}
	if message.Usage != nil && len(message.Usage.Metadata) != 0 {
		return nil, errors.New("Gemini Live usage contains provider-specific metadata")
	}

	var events []llm.StreamEvent
	content := message.ServerContent
	if content != nil && len(content.Text) != 0 && len(content.Audio) != 0 {
		return nil, errors.New("Gemini Live mixed text/audio output is not portable to OpenAI Realtime")
	}
	if content != nil && (len(content.Text) != 0 || len(content.Audio) != 0 || content.GenerationComplete || content.TurnComplete || content.Interrupted) {
		if !d.responseOpen {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStart, Model: d.model})
			d.responseOpen = true
		}
		if len(content.Text) != 0 {
			if d.contentOpen && d.contentKind != "text" {
				return nil, errors.New("Gemini Live changed output modality mid-content")
			}
			if !d.contentOpen {
				events = append(events, llm.StreamEvent{Type: llm.StreamEventContentStart, Index: 0, Block: llm.TextBlock{}})
				d.contentOpen = true
				d.contentKind = "text"
			}
			for _, text := range content.Text {
				events = append(events, llm.StreamEvent{Type: llm.StreamEventTextDelta, Index: 0, TextDelta: text})
			}
		}
		if len(content.Audio) != 0 {
			if d.contentOpen && d.contentKind != "audio" {
				return nil, errors.New("Gemini Live changed output modality mid-content")
			}
			if !d.contentOpen {
				events = append(events, llm.StreamEvent{Type: llm.StreamEventContentStart, Index: 0, Block: llm.AudioBlock{MediaType: geminiLiveOutputAudioMediaType}})
				d.contentOpen = true
				d.contentKind = "audio"
			}
			for _, audio := range content.Audio {
				if audio.MediaType != geminiLiveOutputAudioMediaType {
					return nil, fmt.Errorf("Gemini Live audio output has unsupported MIME type %q", audio.MediaType)
				}
				if _, err := base64.StdEncoding.DecodeString(audio.Data); err != nil {
					return nil, fmt.Errorf("Gemini Live audio output is not valid base64: %w", err)
				}
				events = append(events, llm.StreamEvent{Type: llm.StreamEventAudioDelta, Index: 0, AudioDelta: audio.Data, AudioMediaType: audio.MediaType})
			}
		}
	}

	if len(message.ToolCall) != 0 {
		if d.contentOpen {
			return nil, errors.New("Gemini Live tool call arrived while content was open")
		}
		toolEvents, err := d.decodeToolCalls(message.ToolCall)
		if err != nil {
			return nil, err
		}
		events = append(events, toolEvents...)
	}

	if message.Usage != nil {
		if !d.responseOpen {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStart, Model: d.model})
			d.responseOpen = true
		}
		usage := *message.Usage
		events = append(events, llm.StreamEvent{Type: llm.StreamEventUsage, Usage: &usage})
	}

	if content != nil && (content.TurnComplete || content.Interrupted) {
		if d.contentOpen {
			events = append(events, llm.StreamEvent{Type: llm.StreamEventContentStop, Index: 0})
			d.contentOpen = false
			d.contentKind = ""
		}
		stopReason := llm.StopReasonEndTurn
		if content.Interrupted {
			stopReason = llm.StopReasonUnknown
		}
		events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: stopReason})
		d.responseOpen = false
	}

	if content == nil && message.Usage == nil && len(message.ToolCall) == 0 && len(events) == 0 {
		return nil, fmt.Errorf("Gemini Live server message has no portable realtime content")
	}
	return events, nil
}

func (d *LiveStreamDecoder) decodeToolCalls(data json.RawMessage) ([]llm.StreamEvent, error) {
	var wire liveToolCallEnvelope
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("decode Gemini Live tool call: %w", err)
	}
	if len(wire.FunctionCalls) == 0 {
		return nil, errors.New("Gemini Live tool call contains no function calls")
	}
	var events []llm.StreamEvent
	if !d.responseOpen {
		events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStart, Model: d.model})
		d.responseOpen = true
	}
	for index, call := range wire.FunctionCalls {
		if call.ID == "" || call.Name == "" {
			return nil, errors.New("Gemini Live function call is missing id or name")
		}
		args := call.Args
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		if !json.Valid(args) {
			return nil, fmt.Errorf("Gemini Live function call %q has invalid arguments", call.Name)
		}
		events = append(events,
			llm.StreamEvent{Type: llm.StreamEventToolCallStart, Index: index, Block: llm.ToolCallBlock{ID: call.ID, Name: call.Name, Arguments: json.RawMessage(`{}`)}},
			llm.StreamEvent{Type: llm.StreamEventToolCallDelta, Index: index, ToolCallDelta: &llm.ToolCallDelta{ID: call.ID, Name: call.Name, ArgumentsDelta: string(args)}},
			llm.StreamEvent{Type: llm.StreamEventContentStop, Index: index},
		)
	}
	events = append(events, llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonToolUse})
	d.responseOpen = false
	return events, nil
}
