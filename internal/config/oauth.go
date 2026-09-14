package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type OAuthProfile struct {
	Kind                string            `json:"kind,omitempty"`
	ProviderID          string            `json:"provider_id"`
	ClientID            string            `json:"client_id"`
	AuthorizationURL    string            `json:"authorization_url,omitempty"`
	TokenURL            string            `json:"token_url,omitempty"`
	Scopes              []string          `json:"scopes,omitempty"`
	AuthorizationParams map[string]string `json:"authorization_params,omitempty"`
}

type OAuth struct {
	PublicBaseURL  string
	GeminiClientID string
	Profiles       []OAuthProfile
}

func LoadOAuth() (OAuth, bool, error) {
	cfg := OAuth{
		PublicBaseURL:  strings.TrimRight(strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL")), "/"),
		GeminiClientID: strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID")),
	}
	if raw := strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_PROFILES_JSON")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg.Profiles); err != nil {
			return OAuth{}, false, fmt.Errorf("parse KOKEKOKKOR_OAUTH_PROFILES_JSON: %w", err)
		}
		for i := range cfg.Profiles {
			profile := &cfg.Profiles[i]
			profile.Kind = strings.ToLower(strings.TrimSpace(profile.Kind))
			if profile.Kind == "" {
				profile.Kind = "generic"
			}
			profile.ProviderID = strings.TrimSpace(profile.ProviderID)
			profile.ClientID = strings.TrimSpace(profile.ClientID)
			profile.AuthorizationURL = strings.TrimSpace(profile.AuthorizationURL)
			profile.TokenURL = strings.TrimSpace(profile.TokenURL)
			if profile.Kind != "generic" && profile.Kind != "gemini" {
				return OAuth{}, false, fmt.Errorf("OAuth profile %d has unsupported kind %q", i, profile.Kind)
			}
			if profile.ProviderID == "" || profile.ClientID == "" {
				return OAuth{}, false, fmt.Errorf("OAuth profile %d requires provider_id and client_id", i)
			}
			if profile.Kind == "generic" && (profile.AuthorizationURL == "" || profile.TokenURL == "") {
				return OAuth{}, false, fmt.Errorf("OAuth profile %q requires authorization_url and token_url", profile.ProviderID)
			}
		}
	}

	if cfg.PublicBaseURL == "" && cfg.GeminiClientID == "" && len(cfg.Profiles) == 0 {
		return OAuth{}, false, nil
	}
	if cfg.PublicBaseURL == "" {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL is required when OAuth is enabled")
	}
	if cfg.GeminiClientID == "" && len(cfg.Profiles) == 0 {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID or KOKEKOKKOR_OAUTH_PROFILES_JSON is required when OAuth is enabled")
	}
	parsed, err := url.Parse(cfg.PublicBaseURL)
	if err != nil || parsed.Host == "" {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL must be an absolute URL")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && oauthLoopback(parsed.Hostname())) {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL must use HTTPS except for loopback hosts")
	}
	return cfg, true, nil
}

func oauthLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
