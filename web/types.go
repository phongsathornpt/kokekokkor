package web

// DashboardData holds the state required to render the gateway administration dashboard.
type DashboardData struct {
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
	APIKey         bool
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
