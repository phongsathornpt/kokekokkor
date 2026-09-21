package web

import (
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
)

// PageName identifies the current admin page view.
type PageName string

const (
	PageOverview  PageName = "overview"
	PageProviders PageName = "providers"
	PageDefaults  PageName = "defaults"
	PageRoutes    PageName = "routes"
)

// DashboardData holds the state required to render the gateway administration dashboard.
type DashboardData struct {
	ActivePage         PageName
	Providers          []ProviderView
	Defaults           []DefaultView
	Routes             []RouteView
	CSRF               string
	Editable           bool
	CredentialEditable bool
	Protocols          []string
}

// ModelView represents a model available under a provider.
type ModelView struct {
	ID            string
	Name          string
	UpstreamModel string
	Description   string
	Capabilities  []string
	IsDefault     bool
}

// EffectiveUpstreamModel returns the upstream model identifier or falls back to ID.
func (m ModelView) EffectiveUpstreamModel() string {
	if m.UpstreamModel != "" {
		return m.UpstreamModel
	}
	return m.ID
}

// ProviderView represents a single provider target configuration in the dashboard.
type ProviderView struct {
	ID             string
	Protocol       string
	BaseURL        string
	Enabled        bool
	OAuthAvailable bool
	OAuthConnected bool
	OAuthFlowType  string
	APIKey         bool
	Models         []ModelView
}

// ProviderPreset represents a pre-configured provider target with known endpoints.
type ProviderPreset struct {
	ID          string
	Name        string
	Protocol    string
	BaseURL     string
	Description string
	HasOAuth    bool
	KeyPrefix   string
	Models      []ModelView
}

// ToModelViews converts domain catalog models to ModelView presentation structs.
func ToModelViews(models []domaincatalog.Model) []ModelView {
	views := make([]ModelView, len(models))
	for i, m := range models {
		caps := make([]string, len(m.Capabilities))
		for j, c := range m.Capabilities {
			caps[j] = string(c)
		}
		views[i] = ModelView{
			ID:            m.ID,
			Name:          m.Name,
			UpstreamModel: m.UpstreamModel,
			Description:   m.Description,
			Capabilities:  caps,
			IsDefault:     m.IsDefault,
		}
	}
	return views
}

// DefaultProviderPresets returns the standard list of 1-click provider presets.
func DefaultProviderPresets() []ProviderPreset {
	presets := []ProviderPreset{
		{
			ID:          "openai",
			Name:        "OpenAI",
			Protocol:    "openai",
			BaseURL:     "https://api.openai.com/v1",
			Description: "GPT-5.6 Sol, Terra, Luna & OpenAI endpoints",
			HasOAuth:    true,
			KeyPrefix:   "sk-proj-...",
		},
		{
			ID:          "anthropic",
			Name:        "Anthropic",
			Protocol:    "anthropic",
			BaseURL:     "https://api.anthropic.com",
			Description: "Claude Sonnet 5, Opus 5 & Haiku 4.5",
			HasOAuth:    true,
			KeyPrefix:   "sk-ant-...",
		},
		{
			ID:          "gemini",
			Name:        "Google Gemini",
			Protocol:    "gemini",
			BaseURL:     "https://generativelanguage.googleapis.com",
			Description: "Gemini 3.8 Flash, 3.1 Pro & Flash-Lite",
			HasOAuth:    true,
			KeyPrefix:   "AIza...",
		},
		{
			ID:          "antigravity",
			Name:        "Antigravity (Google Cloud Code)",
			Protocol:    "gemini",
			BaseURL:     "https://cloudcode-pa.googleapis.com",
			Description: "Claude Opus 4.6 & Gemini 3 Pro via Cloud Code Private API",
			HasOAuth:    true,
			KeyPrefix:   "ya29...",
		},
		{
			ID:          "deepseek",
			Name:        "DeepSeek",
			Protocol:    "openai",
			BaseURL:     "https://api.deepseek.com/v1",
			Description: "DeepSeek V4 Pro & V4 Flash reasoning",
			HasOAuth:    false,
			KeyPrefix:   "sk-...",
		},
		{
			ID:          "groq",
			Name:        "Groq",
			Protocol:    "openai",
			BaseURL:     "https://api.groq.com/openai/v1",
			Description: "Ultra-fast open-weight model inference",
			HasOAuth:    false,
			KeyPrefix:   "gsk_...",
		},
		{
			ID:          "openrouter",
			Name:        "OpenRouter",
			Protocol:    "openai",
			BaseURL:     "https://openrouter.ai/api/v1",
			Description: "Unified gateway across 200+ models",
			HasOAuth:    false,
			KeyPrefix:   "sk-or-...",
		},
		{
			ID:          "ollama",
			Name:        "Ollama (Local)",
			Protocol:    "openai",
			BaseURL:     "http://localhost:11434/v1",
			Description: "Local models running without API key",
			HasOAuth:    false,
			KeyPrefix:   "not required",
		},
	}

	for i := range presets {
		presets[i].Models = ToModelViews(domaincatalog.DefaultModelsForProvider(presets[i].ID))
	}
	return presets
}

func PresetOAuthFlowType(providerID string) string {
	if providerID == "openai" || providerID == "github" {
		return "device_code"
	}
	return "authorization_code"
}

// DefaultView represents protocol-level default routing configuration.
type DefaultView struct {
	Protocol          string
	ProviderID        string
	EligibleProviders []string
}

// RouteView represents a model alias mapping to an ordered fallback target pipeline.
type RouteView struct {
	Model      string
	Targets    []string
	TargetSpec string
}

// LoginData holds the view state for the admin authentication page.
type LoginData struct {
	Message string
}
