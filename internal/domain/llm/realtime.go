package llm

import "encoding/json"

// RealtimeEventType identifies portable client-side controls used by live or
// realtime model sessions. Protocol adapters may accept multiple provider wire
// event names for the same semantic operation.
type RealtimeEventType string

const (
	RealtimeEventUnknown          RealtimeEventType = "unknown"
	RealtimeEventSessionStart     RealtimeEventType = "session_start"
	RealtimeEventSessionUpdate    RealtimeEventType = "session_update"
	RealtimeEventInputAudioAppend RealtimeEventType = "input_audio_append"
	RealtimeEventInputAudioCommit RealtimeEventType = "input_audio_commit"
	RealtimeEventInputAudioClear  RealtimeEventType = "input_audio_clear"
	RealtimeEventItemCreate       RealtimeEventType = "item_create"
	RealtimeEventResponseCreate   RealtimeEventType = "response_create"
	RealtimeEventResponseCancel   RealtimeEventType = "response_cancel"
)

type RealtimeSessionConfig struct {
	Model            string
	Instructions     string
	OutputModalities []string
	Metadata         map[string]json.RawMessage
}

// RealtimeEvent keeps the provider wire payload alongside the portable subset.
// Translation code must reject provider-specific fields it cannot represent
// rather than silently discarding them.
type RealtimeEvent struct {
	Type          RealtimeEventType
	EventID       string
	WireType      string
	Session       json.RawMessage
	SessionConfig *RealtimeSessionConfig
	Item          json.RawMessage
	Message       *Message
	Audio         string
	Response      json.RawMessage
	Metadata      map[string]json.RawMessage
}
