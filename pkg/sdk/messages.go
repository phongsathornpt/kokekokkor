package sdk

import (
	"context"
	"fmt"
	"net/http"
)

// ContentBlock represents a block of content in an Anthropic message.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicMessage represents a message in the Anthropic Messages API.
type AnthropicMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// MessagesRequest represents a request to the /v1/messages endpoint.
type MessagesRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

// MessagesResponse represents the response from /v1/messages.
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Messages sends an Anthropic Messages request to the gateway via /v1/messages.
func (c *Client) Messages(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: messages request cannot be nil", ErrBadRequest)
	}
	body, err := jsonBody(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL.String()+"/v1/messages", body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("X-Api-Key", c.apiKey)
	}
	httpReq.Header.Set("Anthropic-Version", "2023-06-01")

	var resp MessagesResponse
	if err := c.doJSON(httpReq, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
