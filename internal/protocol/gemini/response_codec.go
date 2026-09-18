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
	metadata, err := topLevelMetadata(data, "candidates", "usageMetadata", "modelVersion", "responseId")
	if err != nil {
		return llm.Response{}, err
	}
	if raw := nestedExtras(data, "usageMetadata", "promptTokenCount", "candidatesTokenCount", "cachedContentTokenCount", "thoughtsTokenCount", "totalTokenCount"); len(raw) != 0 {
		metadata = putMetadata(metadata, "gemini.usageMetadata", raw)
	}
	if len(wire.Candidates) != 1 {
		return llm.Response{}, fmt.Errorf("Gemini response has %d candidates; cross-protocol translation requires exactly one", len(wire.Candidates))
	}
	candidate := wire.Candidates[0]
	var envelope struct {
		Candidates []json.RawMessage `json:"candidates"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return llm.Response{}, fmt.Errorf("decode Gemini candidate metadata: %w", err)
	}
	var candidateMetadata map[string]json.RawMessage
	if len(envelope.Candidates) == 1 {
		candidateMetadata, err = topLevelMetadata(envelope.Candidates[0], "content", "finishReason")
		if err != nil {
			return llm.Response{}, err
		}
	}
	content, partMetadata, err := decodeGeminiParts(candidate.Content.Parts, 0, make(map[string]string))
	if err != nil {
		return llm.Response{}, fmt.Errorf("decode Gemini candidate: %w", err)
	}
	for key, value := range partMetadata {
		metadata = putMetadata(metadata, key, value)
	}
	if raw := candidateMetadata["groundingMetadata"]; len(raw) != 0 {
		grounded, consumed, err := applyGeminiGrounding(content, raw)
		if err != nil {
			return llm.Response{}, err
		}
		content = grounded
		if consumed {
			delete(candidateMetadata, "groundingMetadata")
		}
	}
	if len(candidateMetadata) != 0 {
		raw, err := json.Marshal(candidateMetadata)
		if err != nil {
			return llm.Response{}, err
		}
		metadata = putMetadata(metadata, "gemini.candidate", raw)
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

func applyGeminiGrounding(content []llm.ContentBlock, raw json.RawMessage) ([]llm.ContentBlock, bool, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, false, fmt.Errorf("decode Gemini groundingMetadata: %w", err)
	}
	for key := range object {
		if key != "groundingChunks" && key != "groundingSupports" {
			return content, false, nil
		}
	}

	var grounding struct {
		Chunks []struct {
			Web *struct {
				URI   string `json:"uri"`
				Title string `json:"title"`
			} `json:"web,omitempty"`
		} `json:"groundingChunks"`
		Supports []struct {
			GroundingChunkIndices []int `json:"groundingChunkIndices"`
			Segment struct {
				PartIndex  int `json:"partIndex"`
				StartIndex int `json:"startIndex"`
				EndIndex   int `json:"endIndex"`
			} `json:"segment"`
		} `json:"groundingSupports"`
	}
	if err := json.Unmarshal(raw, &grounding); err != nil {
		return nil, false, fmt.Errorf("decode Gemini grounding metadata: %w", err)
	}

	out := append([]llm.ContentBlock(nil), content...)
	for _, support := range grounding.Supports {
		if support.Segment.PartIndex < 0 || support.Segment.PartIndex >= len(out) {
			return nil, false, fmt.Errorf("Gemini grounding segment partIndex %d is out of range", support.Segment.PartIndex)
		}
		text, ok := out[support.Segment.PartIndex].(llm.TextBlock)
		if !ok {
			return nil, false, fmt.Errorf("Gemini grounding segment references non-text part %d", support.Segment.PartIndex)
		}
		start, err := runeIndexForByteOffset(text.Text, support.Segment.StartIndex)
		if err != nil {
			return nil, false, err
		}
		end, err := runeIndexForByteOffset(text.Text, support.Segment.EndIndex)
		if err != nil {
			return nil, false, err
		}
		for _, index := range support.GroundingChunkIndices {
			if index < 0 || index >= len(grounding.Chunks) || grounding.Chunks[index].Web == nil || grounding.Chunks[index].Web.URI == "" {
				return content, false, nil
			}
			web := grounding.Chunks[index].Web
			text.Citations = append(text.Citations, llm.URLCitation{
				StartIndex: start,
				EndIndex:   end,
				URL:        web.URI,
				Title:      web.Title,
			})
		}
		out[support.Segment.PartIndex] = text
	}
	return out, true, nil
}

func runeIndexForByteOffset(text string, offset int) (int, error) {
	if offset < 0 || offset > len(text) {
		return 0, fmt.Errorf("Gemini grounding byte offset %d is out of range", offset)
	}
	if offset != len(text) && offset > 0 && (text[offset]&0xc0) == 0x80 {
		return 0, fmt.Errorf("Gemini grounding byte offset %d splits a UTF-8 code point", offset)
	}
	return len([]rune(text[:offset])), nil
}
