package translator

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRealtimeSessionBridgeAudioRoundTrip(t *testing.T) {
	bridge := NewRealtimeSessionBridge("gemini-live-target")
	setup, err := bridge.ClientMessage([]byte(`{"type":"session.update","session":{"output_modalities":["audio"],"input_audio_format":"pcm16","output_audio_format":"pcm16"}}`))
	if err != nil {
		t.Fatalf("setup error = %v", err)
	}
	if len(setup) != 1 || !strings.Contains(string(setup[0]), `"responseModalities":["AUDIO"]`) {
		t.Fatalf("setup = %q", setup)
	}
	if _, err := bridge.ServerMessage([]byte(`{"setupComplete":{}}`)); err != nil {
		t.Fatalf("setupComplete error = %v", err)
	}

	input := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})
	messages, err := bridge.ClientMessage([]byte(`{"type":"input_audio_buffer.append","audio":"` + input + `"}`))
	if err != nil {
		t.Fatalf("audio append error = %v", err)
	}
	if len(messages) != 1 || !strings.Contains(string(messages[0]), `"mimeType":"audio/pcm;rate=24000"`) {
		t.Fatalf("Gemini audio input = %q", messages)
	}
	messages, err = bridge.ClientMessage([]byte(`{"type":"input_audio_buffer.commit"}`))
	if err != nil {
		t.Fatalf("audio commit error = %v", err)
	}
	if len(messages) != 1 || !strings.Contains(string(messages[0]), `"audioStreamEnd":true`) {
		t.Fatalf("Gemini audio commit = %q", messages)
	}

	output := base64.StdEncoding.EncodeToString([]byte{5, 6, 7, 8})
	messages, err = bridge.ServerMessage([]byte(`{"serverContent":{"modelTurn":{"parts":[{"inlineData":{"mimeType":"audio/pcm;rate=24000","data":"` + output + `"}}]}}}`))
	if err != nil {
		t.Fatalf("audio output error = %v", err)
	}
	joined := ""
	for _, payload := range messages {
		joined += string(payload) + "\n"
	}
	if !strings.Contains(joined, "response.output_audio.delta") || !strings.Contains(joined, `"delta":"`+output+`"`) {
		t.Fatalf("OpenAI audio events = %s", joined)
	}

	messages, err = bridge.ServerMessage([]byte(`{"serverContent":{"turnComplete":true}}`))
	if err != nil {
		t.Fatalf("audio turn complete error = %v", err)
	}
	joined = ""
	for _, payload := range messages {
		joined += string(payload) + "\n"
	}
	if !strings.Contains(joined, "response.output_audio.done") || !strings.Contains(joined, "response.done") {
		t.Fatalf("OpenAI audio completion = %s", joined)
	}
}
