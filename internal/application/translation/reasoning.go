package translation

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func openAIReasoningToAnthropic(reasoning *llm.ReasoningConfig) (*llm.ReasoningConfig, error) {
	if reasoning == nil {
		return nil, nil
	}
	if err := rejectReasoningMetadata(reasoning.Metadata); err != nil {
		return nil, err
	}
	result := cloneReasoning(reasoning)
	result.Metadata = nil
	if result.Summary == "" {
		result.Summary = "none"
	}
	if err := validatePortableSummary(result.Summary); err != nil {
		return nil, err
	}

	switch strings.ToLower(result.Effort) {
	case "":
	case "none":
		result.Enabled = false
		result.Mode = "disabled"
		result.Effort = ""
		result.Summary = "none"
		return result, nil
	case "minimal":
		result.Effort = "low"
	case "low", "medium", "high", "xhigh", "max":
		result.Effort = strings.ToLower(result.Effort)
	default:
		return nil, unsupported("reasoning effort", "Anthropic cannot represent OpenAI effort "+result.Effort)
	}
	result.Enabled = true
	if result.BudgetTokens > 0 {
		result.Mode = "enabled"
	} else {
		result.Mode = "adaptive"
	}
	return result, nil
}

func openAIReasoningToGemini(reasoning *llm.ReasoningConfig) (*llm.ReasoningConfig, error) {
	if reasoning == nil {
		return nil, nil
	}
	if err := rejectReasoningMetadata(reasoning.Metadata); err != nil {
		return nil, err
	}
	result := cloneReasoning(reasoning)
	result.Metadata = nil
	if result.Summary == "" {
		result.Summary = "none"
	}
	if err := validatePortableSummary(result.Summary); err != nil {
		return nil, err
	}

	switch strings.ToLower(result.Effort) {
	case "":
	case "none":
		result.Enabled = false
		result.Mode = "disabled"
		result.Effort = "none"
		result.Summary = "none"
	case "minimal", "low", "medium", "high":
		result.Enabled = true
		result.Effort = strings.ToLower(result.Effort)
	case "xhigh", "max":
		result.Enabled = true
		result.Effort = "high"
	default:
		return nil, unsupported("reasoning effort", "Gemini cannot represent OpenAI effort "+result.Effort)
	}
	return result, nil
}

func stripReasoningBlocks(content []llm.ContentBlock) []llm.ContentBlock {
	result := make([]llm.ContentBlock, 0, len(content))
	for _, block := range content {
		if _, ok := block.(llm.ReasoningBlock); ok {
			continue
		}
		result = append(result, block)
	}
	return result
}

func reasoningSummariesForResponses(content []llm.ContentBlock) ([]llm.ContentBlock, error) {
	result := make([]llm.ContentBlock, 0, len(content))
	for _, block := range content {
		reasoning, ok := block.(llm.ReasoningBlock)
		if !ok {
			result = append(result, block)
			continue
		}
		if err := rejectReasoningMetadata(reasoning.Metadata); err != nil {
			return nil, err
		}
		if reasoning.Text == "" {
			continue
		}
		result = append(result, llm.ReasoningBlock{Text: reasoning.Text})
	}
	return result, nil
}

func validatePortableSummary(summary string) error {
	switch summary {
	case "", "none", "auto", "concise", "detailed":
		return nil
	default:
		return unsupported("reasoning summary", "unsupported summary mode "+summary)
	}
}

func rejectReasoningMetadata(metadata map[string]json.RawMessage) error {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return unsupported("reasoning extensions", "unsupported fields: "+strings.Join(keys, ", "))
}

func cloneReasoning(source *llm.ReasoningConfig) *llm.ReasoningConfig {
	if source == nil {
		return nil
	}
	result := *source
	if len(source.Metadata) != 0 {
		result.Metadata = make(map[string]json.RawMessage, len(source.Metadata))
		for key, value := range source.Metadata {
			result.Metadata[key] = append(json.RawMessage(nil), value...)
		}
	}
	return &result
}
