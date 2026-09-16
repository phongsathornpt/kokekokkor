package sse

import (
	"bytes"
	"testing"
)

func FuzzDecode(f *testing.F) {
	f.Add([]byte("event: delta\ndata: hello\n\n"))
	f.Add([]byte("data: first\ndata: second\n\n"))
	f.Add([]byte(": keepalive\n\n"))

	f.Fuzz(func(t *testing.T, input []byte) {
		err := Decode(bytes.NewReader(input), func(event Event) error {
			if len(event.Data) > maxEventDataBytes {
				t.Fatalf("decoded event data = %d bytes, limit = %d", len(event.Data), maxEventDataBytes)
			}
			return nil
		})
		_ = err
	})
}
