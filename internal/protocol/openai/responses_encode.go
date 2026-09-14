package openai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func EncodeResponsesResponse(response llm.Response) ([]byte, error) {
	status := "completed"
	var incomplete any
	switch response.StopReason {
	case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse, llm.StopReasonContentBlock:
	case llm.StopReasonMaxTokens:
		status = "incomplete"
		incomplete = map[string]string{"reason": "max_output_tokens"}
	default:
		return nil, fmt.Errorf("unsupported canonical stop reason %q for Responses", response.StopReason)
	}

	output, outputText, err := encodeResponsesOutput(response)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	wire := responseObject{
		ID:                response.ID,
		Object:            "response",
		CreatedAt:         now,
		Status:            status,
		Error:             nil,
		IncompleteDetails: incomplete,
		Model:             response.Model,
		Output:            output,
		OutputText:        outputText,
		Usage: responseUsage{
			InputTokens: response.Usage.InputTokens,
			InputTokensDetails: responseInputTokenDetails{
				CachedTokens:     response.Usage.CacheReadTokens,
				CacheWriteTokens: response.Usage.CacheWriteTokens,
			},
			OutputTokens: response.Usage.OutputTokens,
			OutputTokensDetail: responseOutputTokenDetails{
				ReasoningTokens: response.Usage.ReasoningTokens,
			},
			TotalTokens: response.Usage.InputTokens + response.Usage.OutputTokens,
		},
	}
	if status == "completed" {
		wire.CompletedAt = now
	}

	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI Responses response: %w", err)
	}
	return encoded, nil
}

func encodeResponsesOutput(response llm.Response) ([]responseOutputItem, string, error) {
	items := make([]responseOutputItem, 0, len(response.Content))
	var allText []string
	var textParts []responseOutputPart
	messageIndex := 0
	callIndex := 0
	reasoningIndex := 0
	isRefusal := response.StopReason == llm.StopReasonContentBlock

	flushText := func() {
		if len(textParts) == 0 {
			return
		}
		items = append(items, responseOutputItem{
			ID:      fmt.Sprintf("msg_gateway_%d", messageIndex),
			Type:    "message",
			Status:  "completed",
			Role:    "assistant",
			Content: append([]responseOutputPart(nil), textParts...),
		})
		messageIndex++
		textParts = textParts[:0]
	}

	for _, block := range response.Content {
		switch value := block.(type) {
		case llm.TextBlock:
			if isRefusal {
				textParts = append(textParts, responseOutputPart{
					Type:    "refusal",
					Refusal: value.Text,
				})
				continue
			}
			textParts = append(textParts, responseOutputPart{
				Type:        "output_text",
				Text:        value.Text,
				Annotations: []any{},
			})
			allText = append(allText, value.Text)

		case llm.ToolCallBlock:
			flushText()
			items = append(items, responseOutputItem{
				ID:        fmt.Sprintf("fc_gateway_%d", callIndex),
				Type:      "function_call",
				Status:    "completed",
				CallID:    value.ID,
				Name:      value.Name,
				Arguments: string(value.Arguments),
			})
			callIndex++

		case llm.ReasoningBlock:
			flushText()
			if value.Text == "" {
				continue
			}
			items = append(items, responseOutputItem{
				ID:     fmt.Sprintf("rs_gateway_%d", reasoningIndex),
				Type:   "reasoning",
				Status: "completed",
				Summary: []responseSummaryPart{{
					Type: "summary_text",
					Text: value.Text,
				}},
			})
			reasoningIndex++

		default:
			return nil, "", fmt.Errorf("canonical response block %T cannot be represented in Responses output", block)
		}
	}
	flushText()
	return items, strings.Join(allText, ""), nil
}
