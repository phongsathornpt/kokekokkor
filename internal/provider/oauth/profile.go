package oauth

import (
	"fmt"
	"strings"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

const (
	GoogleAuthorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL         = "https://oauth2.googleapis.com/token"
)

type ProfileOptions struct {
	ProviderID          string
	ClientID            string
	Scopes              []string
	AuthorizationURL    string
	TokenURL            string
	AuthorizationParams map[string]string
}

func GenericProfile(options ProfileOptions) (domainoauth.Provider, error) {
	provider := domainoauth.Provider{
		ID:                  strings.TrimSpace(options.ProviderID),
		ClientID:            strings.TrimSpace(options.ClientID),
		AuthorizationURL:    strings.TrimSpace(options.AuthorizationURL),
		TokenURL:            strings.TrimSpace(options.TokenURL),
		Scopes:              append([]string(nil), options.Scopes...),
		AuthorizationParams: cloneMap(options.AuthorizationParams),
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
