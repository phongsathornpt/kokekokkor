package translator

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	geminiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
	apptranslation "github.com/phongsathornpt/kokekokkor/internal/usecase/translation"
)

var (
	ErrRealtimeSetupRequired = errors.New("realtime bridge requires session setup before content")
	ErrRealtimeSetupPending  = errors.New("realtime bridge is waiting for Gemini Live setupComplete")
)

// RealtimeSessionBridge owns the protocol state for one OpenAI Realtime ->
// Gemini Live text/function session. Network transports feed complete WebSocket
// text messages into ClientMessage and ServerMessage.
type RealtimeSessionBridge struct {
	mu            sync.Mutex
	clientEncoder *geminiProtocol.LiveClientEncoder
	streamDecoder *geminiProtocol.LiveStreamDecoder
	serverEncoder *openaiProtocol.RealtimeServerEncoder
	setupSent     bool
	setupComplete bool
	toolCalls     map[string]string
}

func NewRealtimeSessionBridge(geminiModel string) *RealtimeSessionBridge {
	return &RealtimeSessionBridge{
		clientEncoder: geminiProtocol.NewLiveClientEncoder(geminiModel),
		streamDecoder: geminiProtocol.NewLiveStreamDecoder(geminiModel),
		serverEncoder: openaiProtocol.NewRealtimeServerEncoder(),
		toolCalls:     make(map[string]string),
	}
}

// ClientMessage translates one OpenAI Realtime client event into Gemini Live
// client messages. The first portable event must configure the session. Gemini
// content is held at the boundary until setupComplete has been observed.
func (b *RealtimeSessionBridge) ClientMessage(data []byte) ([][]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	event, err := openaiProtocol.DecodeRealtimeEvent(data)
	if err != nil {
		return nil, apptranslation.WrapRequest(err)
	}
	if event.ToolResult != nil && event.ToolResult.Name == "" {
		name, ok := b.toolCalls[event.ToolResult.ToolCallID]
		if !ok {
			return nil, apptranslation.WrapRequest(fmt.Errorf("unknown realtime tool call %q", event.ToolResult.ToolCallID))
		}
		event.ToolResult.Name = name
	}
	if err := apptranslation.OpenAIRealtimeToGeminiTextEvent(event); err != nil {
		return nil, err
	}

	isSetup := event.Type == "session_start" || event.Type == "session_update"
	if !b.setupSent && !isSetup {
		return nil, ErrRealtimeSetupRequired
	}
	if b.setupSent && !b.setupComplete && !isSetup {
		return nil, ErrRealtimeSetupPending
	}

	payload, err := b.clientEncoder.Encode(event)
	if err != nil {
		return nil, apptranslation.WrapRequest(err)
	}
	if isSetup {
		b.setupSent = true
	}
	if event.ToolResult != nil {
		delete(b.toolCalls, event.ToolResult.ToolCallID)
	}
	return [][]byte{payload}, nil
}

// ServerMessage translates one Gemini Live server message into OpenAI
// Realtime server events. setupComplete is consumed as bridge state; provider-
// local lifecycle controls fail explicitly until an exact OpenAI mapping exists.
func (b *RealtimeSessionBridge) ServerMessage(data []byte) ([][]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	message, err := geminiProtocol.DecodeLiveServerMessage(data)
	if err != nil {
		return nil, apptranslation.WrapResponse(err)
	}
	if message.SetupComplete {
		if !b.setupSent {
			return nil, apptranslation.WrapResponse(errors.New("Gemini Live sent setupComplete before setup"))
		}
		b.setupComplete = true
	}
	if len(message.GoAway) != 0 {
		return nil, apptranslation.WrapResponse(errors.New("Gemini Live goAway has no portable OpenAI Realtime mapping"))
	}
	if len(message.SessionResumptionUpdate) != 0 {
		return nil, apptranslation.WrapResponse(errors.New("Gemini Live session resumption state has no portable OpenAI Realtime mapping"))
	}
	if len(message.ToolCall) != 0 {
		if err := b.rememberToolCalls(message.ToolCall); err != nil {
			return nil, apptranslation.WrapResponse(err)
		}
	}

	events, err := b.streamDecoder.Decode(message)
	if err != nil {
		return nil, apptranslation.WrapResponse(err)
	}
	var payloads [][]byte
	for _, event := range events {
		encoded, encodeErr := b.serverEncoder.Encode(event)
		if encodeErr != nil {
			return nil, apptranslation.WrapResponse(fmt.Errorf("encode OpenAI Realtime server event: %w", encodeErr))
		}
		payloads = append(payloads, encoded...)
	}
	return payloads, nil
}

func (b *RealtimeSessionBridge) rememberToolCalls(data json.RawMessage) error {
	var wire struct {
		FunctionCalls []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"functionCalls"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode Gemini Live tool-call correlation: %w", err)
	}
	for _, call := range wire.FunctionCalls {
		if call.ID == "" || call.Name == "" {
			return errors.New("Gemini Live function call is missing id or name")
		}
		b.toolCalls[call.ID] = call.Name
	}
	return nil
}

func (b *RealtimeSessionBridge) SetupComplete() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.setupComplete
}
