package openai

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestDecodeRealtimePCM16Session(t *testing.T) {
	event, err := DecodeRealtimeEvent([]byte(`{"type":"session.update","session":{"output_modalities":["audio"],"input_audio_format":"pcm16","output_audio_format":"pcm16"}}`))
	if err != nil {
		t.Fatalf("DecodeRealtimeEvent() error = %v", err)
	}
	if event.SessionConfig == nil {
		t.Fatal("SessionConfig = nil")
	}
	if event.SessionConfig.InputAudioMediaType != "audio/pcm;rate=24000" || event.SessionConfig.OutputAudioMediaType != "audio/pcm;rate=24000" {
		t.Fatalf("audio formats = %#v", event.SessionConfig)
	}
}

func TestRealtimeServerEncoderAudio(t *testing.T) {
	encoder := NewRealtimeServerEncoder()
	data := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})
	var payloads [][]byte
	appendEvents := func(event llm.StreamEvent) {
		events, err := encoder.Encode(event)
		if err != nil {
			t.Fatalf("Encode(%s) error = %v", event.Type, err)
		}
		payloads = append(payloads, events...)
	}
	appendEvents(llm.StreamEvent{Type: llm.StreamEventResponseStart, Model: "gemini-live"})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventContentStart, Index: 0, Block: llm.AudioBlock{MediaType: "audio/pcm;rate=24000"}})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventAudioDelta, Index: 0, AudioDelta: data, AudioMediaType: "audio/pcm;rate=24000"})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventContentStop, Index: 0})
	appendEvents(llm.StreamEvent{Type: llm.StreamEventResponseStop, StopReason: llm.StopReasonEndTurn})

	joined := ""
	for _, payload := range payloads {
		joined += string(payload) + "\n"
	}
	for _, want := range []string{"response.output_audio.delta", "response.output_audio.done", `"delta":"` + data + `"`, `"type":"audio"`, `"output_modalities":["audio"]`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("payloads missing %q:\n%s", want, joined)
		}
	}
}
