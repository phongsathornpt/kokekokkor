package web

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
}

// DefaultProviderPresets returns the standard list of 1-click provider presets.
func DefaultProviderPresets() []ProviderPreset {
	return []ProviderPreset{
		{
			ID:          "openai",
			Name:        "OpenAI",
			Protocol:    "openai",
			BaseURL:     "https://api.openai.com/v1",
			Description: "GPT-4o, o1, o3-mini & OpenAI endpoints",
			HasOAuth:    true,
			KeyPrefix:   "sk-proj-...",
		},
		{
			ID:          "anthropic",
			Name:        "Anthropic",
			Protocol:    "anthropic",
			BaseURL:     "https://api.anthropic.com",
			Description: "Claude 3.7 Sonnet, 3.5 Haiku & Opus",
			HasOAuth:    true,
			KeyPrefix:   "sk-ant-...",
		},
		{
			ID:          "gemini",
			Name:        "Google Gemini",
			Protocol:    "gemini",
			BaseURL:     "https://generativelanguage.googleapis.com",
			Description: "Gemini 2.0 Flash, Pro & Thinking",
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
			Description: "DeepSeek-V3 & DeepSeek-R1 reasoning",
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
