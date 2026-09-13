package llm

import "encoding/json"

type StreamEventType string

const (
	StreamEventResponseStart  StreamEventType = "response_start"
	StreamEventContentStart   StreamEventType = "content_start"
	StreamEventTextDelta      StreamEventType = "text_delta"
	StreamEventReasoningDelta StreamEventType = "reasoning_delta"
	StreamEventToolCallStart  StreamEventType = "tool_call_start"
	StreamEventToolCallDelta  StreamEventType = "tool_call_delta"
	StreamEventContentStop    StreamEventType = "content_stop"
	StreamEventUsage          StreamEventType = "usage"
	StreamEventResponseStop   StreamEventType = "response_stop"
	StreamEventError          StreamEventType = "error"
)

type ToolCallDelta struct {
	ID             string
	Name           string
	ArgumentsDelta string
}

type StreamError struct {
	Code      string
	Message   string
	Retryable bool
	Metadata  map[string]json.RawMessage
}

type StreamEvent struct {
	Type           StreamEventType
	ResponseID     string
	Model          string
	Index          int
	Block          ContentBlock
	TextDelta      string
	ReasoningDelta string
	ToolCallDelta  *ToolCallDelta
	Usage          *Usage
	StopReason     StopReason
	StopSequence   string
	Error          *StreamError
	Metadata       map[string]json.RawMessage
}
