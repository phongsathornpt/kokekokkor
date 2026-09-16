package translator

import (
	"errors"
	"fmt"

	apptranslation "github.com/phongsathornpt/kokekokkor/internal/application/translation"
	geminiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
)

var (
	ErrRealtimeSetupRequired = errors.New("realtime bridge requires session setup before content")
	ErrRealtimeSetupPending  = errors.New("realtime bridge is waiting for Gemini Live setupComplete")
)

// RealtimeSessionBridge owns the protocol state for one OpenAI Realtime ->
// Gemini Live text session. Network transports feed complete WebSocket text
// messages into ClientMessage and ServerMessage; the bridge returns zero or
// more complete text messages for the opposite peer.
type RealtimeSessionBridge struct {
	clientEncoder *geminiProtocol.LiveClientEncoder
	streamDecoder *geminiProtocol.LiveStreamDecoder
	serverEncoder *openaiProtocol.RealtimeServerEncoder
	setupSent     bool
	setupComplete bool
}

func NewRealtimeSessionBridge(geminiModel string) *RealtimeSessionBridge {
	return &RealtimeSessionBridge{
		clientEncoder: geminiProtocol.NewLiveClientEncoder(geminiModel),
		streamDecoder: geminiProtocol.NewLiveStreamDecoder(geminiModel),
		serverEncoder: openaiProtocol.NewRealtimeServerEncoder(),
	}
}

// ClientMessage translates one OpenAI Realtime client event into Gemini Live
// client messages. The first portable event must configure the session. Gemini
// content is held at the boundary until setupComplete has been observed.
func (b *RealtimeSessionBridge) ClientMessage(data []byte) ([][]byte, error) {
	event, err := openaiProtocol.DecodeRealtimeEvent(data)
	if err != nil {
		return nil, apptranslation.WrapRequest(err)
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
	return [][]byte{payload}, nil
}

// ServerMessage translates one Gemini Live server message into OpenAI
// Realtime server events. setupComplete is consumed as bridge state; provider-
// local lifecycle controls fail explicitly until an exact OpenAI mapping exists.
func (b *RealtimeSessionBridge) ServerMessage(data []byte) ([][]byte, error) {
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

func (b *RealtimeSessionBridge) SetupComplete() bool {
	return b.setupComplete
}
