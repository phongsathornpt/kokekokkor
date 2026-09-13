package openai

import "encoding/json"

type chatRequest struct {
	Model               string          `json:"model"`
	Messages            []chatMessage   `json:"messages"`
	Tools               []chatTool      `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	Stop                json.RawMessage `json:"stop,omitempty"`
	ResponseFormat      json.RawMessage `json:"response_format,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
}

type chatMessage struct {
	Role       string          `json:"role"`
	Name       string          `json:"name,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCalls  []chatToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type chatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string          `json:"name"`
	Arguments   string          `json:"arguments,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Description string          `json:"description,omitempty"`
	Strict      bool            `json:"strict,omitempty"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}
