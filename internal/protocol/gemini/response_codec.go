package gemini

import (
	"encoding/json"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func DecodeGenerateContentResponse(data []byte) (llm.Response, error) {
	var wire generateContentResponse
	if err := json.Unmarshal(data, &wire); err != nil {
		return llm.Response{}, fmt.Errorf("decode Gemini generateContent response: %w", err)
	}
	if len(wire.Candidates) != 1 {
		return llm.Response{}, fmt.Errorf("Gemini response has %d candidates; cross-protocol translation requires exactly one", len(wire.Candidates))
	}
	candidate := wire.Candidates[0]
	content, metadata, err := decodeGeminiParts(candidate.Content.Parts, 0, make(map[string]string))
	if err != nil {
		return llm.Response{}, fmt.Errorf("decode Gemini candidate: %w", err)
	}
	response := llm.Response{
		ID:         wire.ResponseID,
		Model:      wire.ModelVersion,
		Content:    content,
		StopReason: decodeGeminiFinishReason(candidate.FinishReason, content),
		Metadata:   metadata,
	}
	if wire.UsageMetadata != nil {
		response.Usage = llm.Usage{
			InputTokens:     wire.UsageMetadata.PromptTokenCount,
			OutputTokens:    wire.UsageMetadata.CandidatesTokenCount,
			CacheReadTokens: wire.UsageMetadata.CachedContentTokenCount,
			ReasoningTokens: wire.UsageMetadata.ThoughtsTokenCount,
		}
	}
	return response, nil
}

func EncodeGenerateContentResponse(response llm.Response) ([]byte, error) {
	parts, err := encodeGeminiParts(response.Content, make(map[string]string))
	if err != nil {
		return nil, fmt.Errorf("encode Gemini response content: %w", err)
	}
	finishReason, err := encodeGeminiFinishReason(response.StopReason)
	if err != nil {
		return nil, err
	}
	wire := generateContentResponse{
		Candidates: []geminiCandidate{{
			Content:      geminiContent{Role: "model", Parts: parts},
			FinishReason: finishReason,
		}},
		UsageMetadata: &geminiUsageMetadata{
			PromptTokenCount:        response.Usage.InputTokens,
			CandidatesTokenCount:    response.Usage.OutputTokens,
			CachedContentTokenCount: response.Usage.CacheReadTokens,
			ThoughtsTokenCount:      response.Usage.ReasoningTokens,
			TotalTokenCount:         response.Usage.InputTokens + response.Usage.OutputTokens,
		},
		ModelVersion: response.Model,
		ResponseID:   response.ID,
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode Gemini generateContent response: %w", err)
	}
	return data, nil
}

func decodeGeminiFinishReason(reason string, content []llm.ContentBlock) llm.StopReason {
	for _, block := range content {
		if _, ok := block.(llm.ToolCallBlock); ok {
			return llm.StopReasonToolUse
		}
	}
	switch reason {
	case "", "STOP":
		return llm.StopReasonEndTurn
	case "MAX_TOKENS":
		return llm.StopReasonMaxTokens
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return llm.StopReasonContentBlock
	default:
		return llm.StopReasonUnknown
	}
}

func encodeGeminiFinishReason(reason llm.StopReason) (string, error) {
	switch reason {
	case llm.StopReasonEndTurn, llm.StopReasonStopSequence, llm.StopReasonToolUse:
		return "STOP", nil
	case llm.StopReasonMaxTokens:
		return "MAX_TOKENS", nil
	case llm.StopReasonContentBlock:
		return "SAFETY", nil
	default:
		return "", fmt.Errorf("unsupported canonical stop reason %q for Gemini", reason)
	}
}
