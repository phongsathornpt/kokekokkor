package oauth

import (
	"fmt"
	"strings"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

const (
	GoogleAuthorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL         = "https://oauth2.googleapis.com/token"

	OpenAIAuthorizationURL       = "https://auth.openai.com/oauth/authorize"
	OpenAITokenURL               = "https://auth.openai.com/oauth/token"
	OpenAIDeviceAuthorizationURL = "https://auth.openai.com/api/accounts/deviceauth/usercode"
	OpenAICodexClientID          = "app_EMoamEEZ73f0CkXaXp7hrann"

	ClaudeAuthorizationURL = "https://claude.ai/oauth/authorize"
	ClaudeTokenURL         = "https://console.anthropic.com/v1/oauth/token"
	ClaudeClientID         = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

	GitHubDeviceAuthorizationURL = "https://github.com/login/device/code"
	GitHubTokenURL               = "https://github.com/login/oauth/access_token"
	GitHubCopilotClientID        = "Iv1.b507a08c87ecfe81"
)

type ProfileOptions struct {
	ProviderID             string
	FlowType               domainoauth.FlowType
	ClientID               string
	Scopes                 []string
	AuthorizationURL       string
	TokenURL               string
	DeviceAuthorizationURL string
	AuthorizationParams    map[string]string
}

func GenericProfile(options ProfileOptions) (domainoauth.Provider, error) {
	provider := domainoauth.Provider{
		ID:                     strings.TrimSpace(options.ProviderID),
		FlowType:               options.FlowType,
		ClientID:               strings.TrimSpace(options.ClientID),
		AuthorizationURL:       strings.TrimSpace(options.AuthorizationURL),
		TokenURL:               strings.TrimSpace(options.TokenURL),
		DeviceAuthorizationURL: strings.TrimSpace(options.DeviceAuthorizationURL),
		Scopes:                 append([]string(nil), options.Scopes...),
		AuthorizationParams:    cloneMap(options.AuthorizationParams),
	}
	if err := provider.Validate(); err != nil {
		return domainoauth.Provider{}, fmt.Errorf("OAuth provider profile: %w", err)
	}
	return provider, nil
}

func GeminiProfile(options ProfileOptions) (domainoauth.Provider, error) {
	defaults := ProfileOptions{
		ProviderID:       "gemini",
		ClientID:         options.ClientID,
		AuthorizationURL: GoogleAuthorizationURL,
		TokenURL:         GoogleTokenURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/generative-language.retriever",
		},
		AuthorizationParams: map[string]string{
			"access_type":            "offline",
			"include_granted_scopes": "true",
			"prompt":                 "consent",
		},
	}
	if value := strings.TrimSpace(options.ProviderID); value != "" {
		defaults.ProviderID = value
	}
	if value := strings.TrimSpace(options.AuthorizationURL); value != "" {
		defaults.AuthorizationURL = value
	}
	if value := strings.TrimSpace(options.TokenURL); value != "" {
		defaults.TokenURL = value
	}
	if len(options.Scopes) != 0 {
		defaults.Scopes = append([]string(nil), options.Scopes...)
	}
	for key, value := range options.AuthorizationParams {
		defaults.AuthorizationParams[key] = value
	}
	return GenericProfile(defaults)
}

func CodexProfile(options ProfileOptions) (domainoauth.Provider, error) {
	clientID := strings.TrimSpace(options.ClientID)
	if clientID == "" || clientID == "default" || clientID == "public" {
		clientID = OpenAICodexClientID
	}
	flowType := options.FlowType
	if flowType == "" {
		flowType = domainoauth.FlowTypeDeviceCode
	}
	defaults := ProfileOptions{
		ProviderID:             "openai",
		FlowType:               flowType,
		ClientID:               clientID,
		AuthorizationURL:       OpenAIAuthorizationURL,
		TokenURL:               OpenAITokenURL,
		DeviceAuthorizationURL: OpenAIDeviceAuthorizationURL,
		Scopes: []string{
			"openid",
			"profile",
			"email",
			"offline_access",
			"model.request",
		},
		AuthorizationParams: map[string]string{
			"id_token_add_organizations": "true",
			"codex_cli_simplified_flow":  "true",
		},
	}
	if value := strings.TrimSpace(options.ProviderID); value != "" {
		defaults.ProviderID = value
	}
	if value := strings.TrimSpace(options.AuthorizationURL); value != "" {
		defaults.AuthorizationURL = value
	}
	if value := strings.TrimSpace(options.TokenURL); value != "" {
		defaults.TokenURL = value
	}
	if value := strings.TrimSpace(options.DeviceAuthorizationURL); value != "" {
		defaults.DeviceAuthorizationURL = value
	}
	if len(options.Scopes) != 0 {
		defaults.Scopes = append([]string(nil), options.Scopes...)
	}
	for key, value := range options.AuthorizationParams {
		defaults.AuthorizationParams[key] = value
	}
	return GenericProfile(defaults)
}

func ChatGPTProfile(options ProfileOptions) (domainoauth.Provider, error) {
	return CodexProfile(options)
}

func ClaudeProfile(options ProfileOptions) (domainoauth.Provider, error) {
	clientID := strings.TrimSpace(options.ClientID)
	if clientID == "" || clientID == "default" || clientID == "public" {
		clientID = ClaudeClientID
	}
	defaults := ProfileOptions{
		ProviderID:       "anthropic",
		ClientID:         clientID,
		AuthorizationURL: ClaudeAuthorizationURL,
		TokenURL:         ClaudeTokenURL,
		Scopes: []string{
			"user:profile",
			"user:sessions:claude_code",
		},
		AuthorizationParams: make(map[string]string),
	}
	if value := strings.TrimSpace(options.ProviderID); value != "" {
		defaults.ProviderID = value
	}
	if value := strings.TrimSpace(options.AuthorizationURL); value != "" {
		defaults.AuthorizationURL = value
	}
	if value := strings.TrimSpace(options.TokenURL); value != "" {
		defaults.TokenURL = value
	}
	if len(options.Scopes) != 0 {
		defaults.Scopes = append([]string(nil), options.Scopes...)
	}
	for key, value := range options.AuthorizationParams {
		defaults.AuthorizationParams[key] = value
	}
	return GenericProfile(defaults)
}

func GitHubCopilotProfile(options ProfileOptions) (domainoauth.Provider, error) {
	clientID := strings.TrimSpace(options.ClientID)
	if clientID == "" || clientID == "default" || clientID == "public" {
		clientID = GitHubCopilotClientID
	}
	defaults := ProfileOptions{
		ProviderID:             "github",
		FlowType:               domainoauth.FlowTypeDeviceCode,
		ClientID:               clientID,
		DeviceAuthorizationURL: GitHubDeviceAuthorizationURL,
		TokenURL:               GitHubTokenURL,
		Scopes: []string{
			"read:user",
		},
		AuthorizationParams: make(map[string]string),
	}
	if value := strings.TrimSpace(options.ProviderID); value != "" {
		defaults.ProviderID = value
	}
	if value := strings.TrimSpace(options.DeviceAuthorizationURL); value != "" {
		defaults.DeviceAuthorizationURL = value
	}
	if value := strings.TrimSpace(options.TokenURL); value != "" {
		defaults.TokenURL = value
	}
	if len(options.Scopes) != 0 {
		defaults.Scopes = append([]string(nil), options.Scopes...)
	}
	for key, value := range options.AuthorizationParams {
		defaults.AuthorizationParams[key] = value
	}
	return GenericProfile(defaults)
}

func cloneMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
