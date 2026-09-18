package openai

type responsesStreamSnapshot struct {
	ID                 string                `json:"id"`
	Object             string                `json:"object"`
	CreatedAt          int64                 `json:"created_at"`
	CompletedAt        *int64                `json:"completed_at"`
	Status             string                `json:"status"`
	Error              any                   `json:"error"`
	IncompleteDetails  any                   `json:"incomplete_details"`
	Instructions       any                   `json:"instructions"`
	MaxOutputTokens    any                   `json:"max_output_tokens"`
	Model              string                `json:"model"`
	Output             []responseOutputItem  `json:"output"`
	OutputText         string                `json:"output_text,omitempty"`
	ParallelToolCalls  bool                  `json:"parallel_tool_calls"`
	PreviousResponseID any                   `json:"previous_response_id"`
	Conversation       *responseConversation `json:"conversation,omitempty"`
	Prompt             any                   `json:"prompt"`
	Reasoning          map[string]any        `json:"reasoning"`
	SafetyIdentifier   any                   `json:"safety_identifier"`
	ServiceTier        string                `json:"service_tier"`
	Store              bool                  `json:"store"`
	Temperature        float64               `json:"temperature"`
	Text               map[string]any        `json:"text"`
	ToolChoice         any                   `json:"tool_choice"`
	Tools              []any                 `json:"tools"`
	TopLogprobs        int                   `json:"top_logprobs"`
	TopP               float64               `json:"top_p"`
	Truncation         string                `json:"truncation"`
	Usage              *responseUsage        `json:"usage"`
	User               any                   `json:"user"`
	Metadata           map[string]string     `json:"metadata"`
}
