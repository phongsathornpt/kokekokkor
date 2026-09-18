package openai

import "encoding/json"

type responsesRequest struct {
	Model              string                   `json:"model"`
	Instructions       json.RawMessage          `json:"instructions,omitempty"`
	Input              json.RawMessage          `json:"input,omitempty"`
	Tools              []responseTool           `json:"tools,omitempty"`
	ToolChoice         json.RawMessage          `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool                    `json:"parallel_tool_calls,omitempty"`
	MaxOutputTokens    *int                     `json:"max_output_tokens,omitempty"`
	Temperature        *float64                 `json:"temperature,omitempty"`
	TopP               *float64                 `json:"top_p,omitempty"`
	Reasoning          *responseReasoningConfig `json:"reasoning,omitempty"`
	Text               json.RawMessage          `json:"text,omitempty"`
	PreviousResponseID string                   `json:"previous_response_id,omitempty"`
	Conversation       json.RawMessage          `json:"conversation,omitempty"`
	Store              *bool                    `json:"store,omitempty"`
	Stream             bool                     `json:"stream,omitempty"`
}

type responseReasoningConfig struct {
	Context         string `json:"context,omitempty"`
	Effort          string `json:"effort,omitempty"`
	GenerateSummary string `json:"generate_summary,omitempty"`
	Mode            string `json:"mode,omitempty"`
	Summary         string `json:"summary,omitempty"`
}

type responseInputItem struct {
	Type      string          `json:"type,omitempty"`
	Role      string          `json:"role,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
}

type responseContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	FileData string `json:"file_data,omitempty"`
	FileURL  string `json:"file_url,omitempty"`
	Filename string `json:"filename,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type responseTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      bool            `json:"strict,omitempty"`
}

type responseConversation struct {
	ID string `json:"id"`
}

type responseObject struct {
	ID                 string                `json:"id"`
	Object             string                `json:"object"`
	CreatedAt          int64                 `json:"created_at"`
	CompletedAt        int64                 `json:"completed_at,omitempty"`
	Status             string                `json:"status"`
	Error              any                   `json:"error"`
	IncompleteDetails  any                   `json:"incomplete_details"`
	Model              string                `json:"model"`
	PreviousResponseID string                `json:"previous_response_id,omitempty"`
	Conversation       *responseConversation `json:"conversation,omitempty"`
	Output             []responseOutputItem  `json:"output"`
	OutputText         string                `json:"output_text,omitempty"`
	Usage              responseUsage         `json:"usage"`
}

type responseWebSearchAction struct {
	Type    string   `json:"type"`
	Queries []string `json:"queries,omitempty"`
}

type responseOutputItem struct {
	ID        string                   `json:"id"`
	Type      string                   `json:"type"`
	Status    string                   `json:"status,omitempty"`
	Role      string                   `json:"role,omitempty"`
	Content   []responseOutputPart     `json:"content,omitempty"`
	Summary   []responseSummaryPart    `json:"summary,omitempty"`
	Action    *responseWebSearchAction `json:"action,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
}

type responseSummaryPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responseURLCitation struct {
	Type       string `json:"type"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
	URL        string `json:"url"`
	Title      string `json:"title"`
}

type responseOutputPart struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Refusal     string `json:"refusal,omitempty"`
	Annotations []any  `json:"annotations,omitempty"`
}

type responseUsage struct {
	InputTokens        int64                      `json:"input_tokens"`
	InputTokensDetails responseInputTokenDetails  `json:"input_tokens_details"`
	OutputTokens       int64                      `json:"output_tokens"`
	OutputTokensDetail responseOutputTokenDetails `json:"output_tokens_details"`
	TotalTokens        int64                      `json:"total_tokens"`
}

type responseInputTokenDetails struct {
	CachedTokens     int64 `json:"cached_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`
}

type responseOutputTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}
