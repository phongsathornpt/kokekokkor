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

// RealtimeEvent is intentionally small. Translation code can reason about the
// portable control operation while retaining provider payloads until an exact
// semantic mapping exists. Provider-specific fields must not be silently
// discarded when crossing protocols.
type RealtimeEvent struct {
	Type     RealtimeEventType
	EventID  string
	WireType string
	Session  json.RawMessage
	Item     json.RawMessage
	Audio    string
	Response json.RawMessage
	Metadata map[string]json.RawMessage
}
