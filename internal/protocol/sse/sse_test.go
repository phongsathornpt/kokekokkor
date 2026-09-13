package sse

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeMultilineData(t *testing.T) {
	input := "event: delta\r\ndata: first\r\ndata: second\r\n\r\n"
	var events []Event
	if err := Decode(strings.NewReader(input), func(event Event) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Name != "delta" || string(events[0].Data) != "first\nsecond" {
		t.Fatalf("event = %#v", events[0])
	}
}

func TestWriteMultilineData(t *testing.T) {
	var output bytes.Buffer
	if err := Write(&output, Event{Name: "delta", Data: []byte("first\nsecond")}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	want := "event: delta\ndata: first\ndata: second\n\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}
