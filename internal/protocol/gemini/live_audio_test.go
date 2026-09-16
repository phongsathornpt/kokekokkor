package gemini

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func TestLiveClientEncoderAudioInput(t *testing.T) {
	encoder := NewLiveClientEncoder("gemini-live")
	setup, err := encoder.Encode(llm.RealtimeEvent{Type: llm.RealtimeEventSessionUpdate, SessionConfig: &llm.RealtimeSessionConfig{
		OutputModalities:    []string{"audio"},
		InputAudioMediaType: "audio/pcm;rate=24000",
	}})
	if err != nil {
		t.Fatalf("setup Encode() error = %v", err)
	}
	if !strings.Contains(string(setup), `"responseModalities":["AUDIO"]`) {
		t.Fatalf("setup = %s", setup)
	}

	audio := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})
	payload, err := encoder.Encode(llm.RealtimeEvent{Type: llm.RealtimeEventInputAudioAppend, Audio: audio})
	if err != nil {
		t.Fatalf("audio append Encode() error = %v", err)
	}
	for _, want := range []string{`"realtimeInput"`, `"mimeType":"audio/pcm;rate=24000"`, `"data":"` + audio + `"`} {
		if !strings.Contains(string(payload), want) {
			t.Fatalf("audio payload missing %q: %s", want, payload)
		}
	}

	payload, err = encoder.Encode(llm.RealtimeEvent{Type: llm.RealtimeEventInputAudioCommit})
	if err != nil {
		t.Fatalf("audio commit Encode() error = %v", err)
	}
	if string(payload) != `{"realtimeInput":{"audioStreamEnd":true}}` {
		t.Fatalf("audio commit = %s", payload)
	}
}

func TestLiveServerAudioToCanonical(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte{5, 6, 7, 8})
	message, err := DecodeLiveServerMessage([]byte(`{"serverContent":{"modelTurn":{"parts":[{"inlineData":{"mimeType":"audio/pcm;rate=24000","data":"` + data + `"}}]}}}`))
	if err != nil {
		t.Fatalf("DecodeLiveServerMessage() error = %v", err)
	}
	if message.ServerContent == nil || len(message.ServerContent.Audio) != 1 {
		t.Fatalf("ServerContent = %#v", message.ServerContent)
	}

	decoder := NewLiveStreamDecoder("gemini-live")
	events, err := decoder.Decode(message)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(events) != 3 || events[0].Type != llm.StreamEventResponseStart || events[1].Type != llm.StreamEventContentStart || events[2].Type != llm.StreamEventAudioDelta {
		t.Fatalf("events = %#v", events)
	}
	block, ok := events[1].Block.(llm.AudioBlock)
	if !ok || block.MediaType != "audio/pcm;rate=24000" {
		t.Fatalf("audio block = %#v", events[1].Block)
	}
	if events[2].AudioDelta != data || events[2].AudioMediaType != "audio/pcm;rate=24000" {
		t.Fatalf("audio delta = %#v", events[2])
	}
}
