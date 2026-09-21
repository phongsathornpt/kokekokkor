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
	case clean == "antigravity" || clean == "agy":
		return AntigravityModels()
	case clean == "openai":
		return []Model{
			{ID: "gpt-5.6-sol", Name: "GPT-5.6 Sol", Description: "Flagship model for complex reasoning and coding", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "gpt-5.6-terra", Name: "GPT-5.6 Terra", Description: "Balanced intelligence, speed, and cost", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode}},
			{ID: "gpt-5.6-luna", Name: "GPT-5.6 Luna", Description: "Cost-efficient model for high-volume workloads", Capabilities: []Capability{CapabilityFast, CapabilityChat, CapabilityCode}},
		}
	case clean == "anthropic":
		return []Model{
			{ID: "claude-sonnet-5", Name: "Claude Sonnet 5", Description: "Balanced frontier model for coding and general workloads", Capabilities: []Capability{CapabilityThinking, CapabilityCode, CapabilityChat}, IsDefault: true},
			{ID: "claude-opus-5", Name: "Claude Opus 5", Description: "Highest-capability Claude model for complex reasoning", Capabilities: []Capability{CapabilityThinking, CapabilityCode, CapabilityChat}},
			{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", Description: "Fast, efficient model for high-throughput workloads", Capabilities: []Capability{CapabilityFast, CapabilityChat}},
		}
	case clean == "gemini":
		return []Model{
			{ID: "gemini-3.8-flash", Name: "Gemini 3.8 Flash", Description: "Stable multimodal model for coding and agentic workloads", Capabilities: []Capability{CapabilityFast, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "gemini-3.1-pro-preview", Name: "Gemini 3.1 Pro", Description: "Preview reasoning model for complex coding and analysis", Capabilities: []Capability{CapabilityThinking, CapabilityCode, CapabilityChat}},
			{ID: "gemini-3.5-flash-lite", Name: "Gemini 3.5 Flash-Lite", Description: "Cost-efficient model for high-throughput workloads", Capabilities: []Capability{CapabilityFast, CapabilityChat}},
		}
	case clean == "deepseek":
		return []Model{
			{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", Description: "Highest-capability DeepSeek model for reasoning and coding", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Description: "Fast, efficient DeepSeek model for general workloads", Capabilities: []Capability{CapabilityFast, CapabilityChat, CapabilityCode}},
		}
	case clean == "groq":
		return []Model{
			{ID: "openai/gpt-oss-120b", Name: "GPT-OSS 120B", Description: "Open-weight reasoning and coding model optimized for Groq speed", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "openai/gpt-oss-20b", Name: "GPT-OSS 20B", Description: "Fast, cost-efficient open-weight model", Capabilities: []Capability{CapabilityFast, CapabilityChat, CapabilityCode}},
			{ID: "qwen/qwen3.8-27b", Name: "Qwen 3.8 27B", Description: "Multimodal reasoning model with tool and JSON support", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode, CapabilityImage}},
		}
	case clean == "openrouter":
		return []Model{
			{ID: "deepseek/deepseek-v4-pro", Name: "DeepSeek V4 Pro", Description: "Frontier reasoning and coding model through OpenRouter", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "deepseek/deepseek-v4-flash", Name: "DeepSeek V4 Flash", Description: "Fast, efficient model through OpenRouter", Capabilities: []Capability{CapabilityFast, CapabilityChat, CapabilityCode}},
			{ID: "openrouter/auto", Name: "OpenRouter Auto", Description: "OpenRouter selects an available model for the request", Capabilities: []Capability{CapabilityChat, CapabilityCode}},
		}
	case clean == "ollama":
		return []Model{
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Description: "Local-capable reasoning model for efficient inference", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode}, IsDefault: true},
			{ID: "glm-5.3", Name: "GLM 5.3", Description: "Open-weight coding and reasoning model", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode}},
			{ID: "qwen3.8-flash-next", Name: "Qwen 3.8 Flash Next", Description: "Experimental multimodal reasoning model", Capabilities: []Capability{CapabilityThinking, CapabilityChat, CapabilityCode, CapabilityImage}},
		}
	default:
		return nil
	}
}
