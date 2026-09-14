package sse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

const maxLineBytes = 4 << 20
const maxEventDataBytes = 8 << 20

type Event struct {
	Name string
	Data []byte
}

func Decode(r io.Reader, emit func(Event) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), maxLineBytes)

	var event Event
	var data bytes.Buffer
	flush := func() error {
		if event.Name == "" && data.Len() == 0 {
			return nil
		}
		payload := append([]byte(nil), data.Bytes()...)
		if err := emit(Event{Name: event.Name, Data: payload}); err != nil {
			return err
		}
		event = Event{}
		data.Reset()
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			field = line
			value = ""
		} else if strings.HasPrefix(value, " ") {
			value = value[1:]
		}

		switch field {
		case "event":
			event.Name = value
		case "data":
			additional := len(value)
			if data.Len() != 0 {
				additional++
			}
			if data.Len()+additional > maxEventDataBytes {
				return fmt.Errorf("SSE event data exceeds %d byte limit", maxEventDataBytes)
			}
			if data.Len() != 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan SSE stream: %w", err)
	}
	return flush()
}

func Write(w io.Writer, event Event) error {
	if event.Name != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event.Name); err != nil {
			return err
		}
	}
	for _, line := range bytes.Split(event.Data, []byte{'\n'}) {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}
