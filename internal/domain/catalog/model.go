package catalog

import "strings"

// Capability denotes specific features or modalities supported by a model.
type Capability string

const (
	CapabilityChat     Capability = "chat"
	CapabilityThinking Capability = "thinking"
	CapabilityCode     Capability = "code"
	CapabilityImage    Capability = "image"
	CapabilityFast     Capability = "fast"
)

// Model defines a known model available under a provider.
type Model struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	UpstreamModel string       `json:"upstream_model,omitempty"`
	Description   string       `json:"description,omitempty"`
	Capabilities  []Capability `json:"capabilities,omitempty"`
	IsDefault     bool         `json:"is_default,omitempty"`
}

// EffectiveUpstreamModel returns the upstream model ID, defaulting to ID if empty.
func (m Model) EffectiveUpstreamModel() string {
	if m.UpstreamModel != "" {
		return m.UpstreamModel
	}
	return m.ID
}

// AntigravityModels returns the curated list of models supported by Antigravity (Google Cloud Code).
func AntigravityModels() []Model {
	return []Model{
		{
			ID:           "claude-opus-4-6-thinking",
			Name:         "Claude Opus 4.6 (Thinking)",
			Description:  "Anthropic Claude Opus 4.6 with adaptive thinking via Google Cloud Code",
			Capabilities: []Capability{CapabilityThinking, CapabilityCode, CapabilityChat},
			IsDefault:    true,
		},
		{
			ID:           "claude-sonnet-4-6",
			Name:         "Claude Sonnet 4.6 (Thinking)",
			Description:  "Anthropic Claude Sonnet 4.6 with thinking support",
			Capabilities: []Capability{CapabilityThinking, CapabilityCode, CapabilityChat},
		},
		{
			ID:            "gemini-3.8-flash-high",
			Name:          "Gemini 3.8 Flash (High)",
			UpstreamModel: "gemini-3.8-flash-high(high)",
			Description:   "Google Gemini 3.8 Flash with high reasoning effort tier",
			Capabilities:  []Capability{CapabilityThinking, CapabilityFast, CapabilityChat},
		},
		{
			ID:            "gemini-3.8-flash-medium",
			Name:          "Gemini 3.8 Flash (Medium)",
			UpstreamModel: "gemini-3.8-flash-medium(medium)",
			Description:   "Google Gemini 3.8 Flash with balanced reasoning effort tier",
			Capabilities:  []Capability{CapabilityThinking, CapabilityFast, CapabilityChat},
		},
		{
			ID:            "gemini-3.8-flash-low",
			Name:          "Gemini 3.8 Flash (Low)",
			UpstreamModel: "gemini-3.8-flash-low(low)",
			Description:   "Google Gemini 3.8 Flash with minimal reasoning effort",
			Capabilities:  []Capability{CapabilityFast, CapabilityChat},
		},
		{
			ID:            "gemini-3.8-flash",
			Name:          "Gemini 3.8 Flash",
			UpstreamModel: "gemini-3.8-flash-medium(medium)",
			Description:   "Default balanced tier of Google Gemini 3.8 Flash",
			Capabilities:  []Capability{CapabilityFast, CapabilityChat},
		},
		{
			ID:            "gemini-3.7-flash-high",
			Name:          "Gemini 3.7 Flash (High)",
			UpstreamModel: "gemini-3.7-flash-tiered(high)",
			Description:   "Google Gemini 3.7 Flash with high reasoning effort",
			Capabilities:  []Capability{CapabilityThinking, CapabilityCode, CapabilityChat},
		},
		{
			ID:            "gemini-3.7-flash-medium",
			Name:          "Gemini 3.7 Flash (Medium)",
			UpstreamModel: "gemini-3.7-flash-tiered(medium)",
			Description:   "Google Gemini 3.7 Flash with balanced reasoning effort",
			Capabilities:  []Capability{CapabilityThinking, CapabilityCode, CapabilityChat},
		},
		{
			ID:            "gemini-3.7-flash-low",
			Name:          "Gemini 3.7 Flash (Low)",
			UpstreamModel: "gemini-3.7-flash-tiered(low)",
			Description:   "Google Gemini 3.7 Flash low latency coding",
			Capabilities:  []Capability{CapabilityFast, CapabilityCode, CapabilityChat},
		},
		{
			ID:            "gemini-3.6-flash-high",
			Name:          "Gemini 3.6 Flash (High)",
			UpstreamModel: "gemini-3.6-flash-tiered(high)",
			Description:   "Google Gemini 3.6 Flash with high reasoning effort",
			Capabilities:  []Capability{CapabilityThinking, CapabilityChat},
		},
		{
			ID:            "gemini-3.5-flash-high",
			Name:          "Gemini 3.5 Flash (High)",
			UpstreamModel: "gemini-3.5-flash-high",
			Description:   "Google Gemini 3.5 Flash high reasoning tier",
			Capabilities:  []Capability{CapabilityThinking, CapabilityFast, CapabilityChat},
		},
		{
			ID:            "gemini-pro-agent",
			Name:          "Gemini 3.1 Pro (High)",
			UpstreamModel: "gemini-pro-agent",
			Description:   "Google Gemini 3.1 Pro deep reasoning model",
			Capabilities:  []Capability{CapabilityThinking, CapabilityCode, CapabilityChat},
		},
		{
			ID:            "gpt-oss-120b-medium",
			Name:          "GPT-OSS 120B (Medium)",
			UpstreamModel: "gpt-oss-120b-medium",
			Description:   "Open-weights 120B parameter model hosted on Cloud Code",
			Capabilities:  []Capability{CapabilityChat},
		},
		{
			ID:            "gemini-3-flash",
			Name:          "Gemini 3 Flash",
			UpstreamModel: "gemini-3-flash",
			Description:   "Google Gemini 3 Flash standard fast generation without thinking",
			Capabilities:  []Capability{CapabilityFast, CapabilityChat},
		},
		{
			ID:            "gemini-3.1-flash-image",
			Name:          "Gemini 3.1 Flash (Image)",
			UpstreamModel: "gemini-3.1-flash-image",
			Description:   "Multimodal image generation endpoint",
			Capabilities:  []Capability{CapabilityImage},
		},
	}
}

// DefaultModelsForProvider returns the known model list for a given provider preset or target ID.
func DefaultModelsForProvider(providerID string) []Model {
	clean := strings.ToLower(strings.TrimSpace(providerID))
	switch {
	case strings.Contains(clean, "antigravity") || strings.Contains(clean, "agy"):
		return AntigravityModels()
	case strings.Contains(clean, "openai"):
		return []Model{
			{ID: "gpt-4o", Name: "GPT-4o", Description: "High-intelligence flagship multimodal model", Capabilities: []Capability{CapabilityFast, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Description: "Fast, lightweight model for everyday tasks", Capabilities: []Capability{CapabilityFast, CapabilityChat}},
			{ID: "o1", Name: "o1", Description: "Reasoning model designed for complex STEM and coding problems", Capabilities: []Capability{CapabilityThinking, CapabilityCode}},
			{ID: "o3-mini", Name: "o3-mini", Description: "Fast, cost-efficient reasoning model", Capabilities: []Capability{CapabilityThinking, CapabilityFast, CapabilityCode}},
		}
	case strings.Contains(clean, "anthropic"):
		return []Model{
			{ID: "claude-3-7-sonnet-20250219", Name: "Claude 3.7 Sonnet", Description: "Anthropic flagship hybrid reasoning model", Capabilities: []Capability{CapabilityThinking, CapabilityCode, CapabilityChat}, IsDefault: true},
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Description: "Industry-leading intelligence and coding benchmark", Capabilities: []Capability{CapabilityCode, CapabilityChat}},
			{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Description: "Fastest Claude model for high-throughput applications", Capabilities: []Capability{CapabilityFast, CapabilityChat}},
		}
	case strings.Contains(clean, "gemini"):
		return []Model{
			{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Description: "Next-gen multimodal model with native tool use", Capabilities: []Capability{CapabilityFast, CapabilityChat}, IsDefault: true},
			{ID: "gemini-2.0-flash-thinking-exp-01-21", Name: "Gemini 2.0 Flash Thinking", Description: "Experimental reasoning model showing thoughts", Capabilities: []Capability{CapabilityThinking, CapabilityChat}},
			{ID: "gemini-2.0-pro-exp-02-05", Name: "Gemini 2.0 Pro", Description: "High capability model for coding and complex prompts", Capabilities: []Capability{CapabilityThinking, CapabilityCode}},
		}
	case strings.Contains(clean, "deepseek"):
		return []Model{
			{ID: "deepseek-chat", Name: "DeepSeek-V3", Description: "DeepSeek general chat & code generation model", Capabilities: []Capability{CapabilityFast, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "deepseek-reasoner", Name: "DeepSeek-R1", Description: "DeepSeek reasoning model with chain-of-thought", Capabilities: []Capability{CapabilityThinking, CapabilityCode}},
		}
	default:
		return nil
	}
}
