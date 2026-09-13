package oauth

import (
	"fmt"
	"strings"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

const (
	OpenAIAuthorizationURL   = "https://auth.openai.com/oauth/authorize"
	OpenAITokenURL           = "https://auth.openai.com/oauth/token"
	AnthropicAuthorizationURL = "https://platform.claude.com/oauth/authorize"
	AnthropicTokenURL         = "https://platform.claude.com/v1/oauth/token"
	GoogleAuthorizationURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL            = "https://oauth2.googleapis.com/token"
)

type ProfileOptions struct {
	ProviderID       string
	ClientID         string
	Scopes           []string
	AuthorizationURL string
	TokenURL         string
}

func OpenAIProfile(options ProfileOptions) (domainoauth.Provider, error) {
	return profile(options, domainoauth.Provider{
		ID:               "openai",
		AuthorizationURL: OpenAIAuthorizationURL,
		TokenURL:         OpenAITokenURL,
		Scopes: []string{
			"openid",
			"profile",
			"email",
			"offline_access",
			"api.connectors.read",
			"api.connectors.invoke",
		},
		AuthorizationParams: map[string]string{
			"id_token_add_organizations": "true",
			"codex_cli_simplified_flow":  "true",
			"originator":                 "kokekokkor",
		},
	})
}

func AnthropicProfile(options ProfileOptions) (domainoauth.Provider, error) {
	return profile(options, domainoauth.Provider{
		ID:               "anthropic",
		AuthorizationURL: AnthropicAuthorizationURL,
		TokenURL:         AnthropicTokenURL,
		Scopes: []string{
			"user:profile",
			"user:inference",
			"user:sessions:claude_code",
		},
	})
}

func GeminiProfile(options ProfileOptions) (domainoauth.Provider, error) {
	return profile(options, domainoauth.Provider{
		ID:               "gemini",
		AuthorizationURL: GoogleAuthorizationURL,
		TokenURL:         GoogleTokenURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/generative-language.retriever",
		},
		AuthorizationParams: map[string]string{
			"access_type":             "offline",
			"include_granted_scopes": "true",
			"prompt":                  "consent",
		},
	})
}

func profile(options ProfileOptions, defaults domainoauth.Provider) (domainoauth.Provider, error) {
	if value := strings.TrimSpace(options.ProviderID); value != "" {
		defaults.ID = value
	}
	defaults.ClientID = strings.TrimSpace(options.ClientID)
	if len(options.Scopes) != 0 {
		defaults.Scopes = append([]string(nil), options.Scopes...)
	} else {
		defaults.Scopes = append([]string(nil), defaults.Scopes...)
	}
	if value := strings.TrimSpace(options.AuthorizationURL); value != "" {
		defaults.AuthorizationURL = value
	}
	if value := strings.TrimSpace(options.TokenURL); value != "" {
		defaults.TokenURL = value
	}
	if defaults.AuthorizationParams != nil {
		params := make(map[string]string, len(defaults.AuthorizationParams))
		for key, value := range defaults.AuthorizationParams {
			params[key] = value
		}
		defaults.AuthorizationParams = params
	}
	if err := defaults.Validate(); err != nil {
		return domainoauth.Provider{}, fmt.Errorf("OAuth provider profile: %w", err)
	}
	return defaults, nil
}
