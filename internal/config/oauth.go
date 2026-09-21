package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	DefaultCodexClientID       = "app_EMoamEEZ73f0CkXaXp7hrann"
	DefaultClaudeClientID      = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	DefaultGitHubClientID      = "Iv1.b507a08c87ecfe81"
	DefaultAntigravityClientID = "REMOVED_GOOGLE_OAUTH_CLIENT_ID"
)

type OAuthProfile struct {
	Kind                   string            `json:"kind,omitempty"`
	ProviderID             string            `json:"provider_id"`
	ClientID               string            `json:"client_id"`
	AuthorizationURL       string            `json:"authorization_url,omitempty"`
	TokenURL               string            `json:"token_url,omitempty"`
	DeviceAuthorizationURL string            `json:"device_authorization_url,omitempty"`
	Scopes                 []string          `json:"scopes,omitempty"`
	AuthorizationParams    map[string]string `json:"authorization_params,omitempty"`
}

type OAuth struct {
	PublicBaseURL       string
	GeminiClientID      string
	CodexClientID       string
	ClaudeClientID      string
	GitHubClientID      string
	AntigravityClientID string
	Profiles            []OAuthProfile
}

func LoadOAuth() (OAuth, bool, error) {
	cfg := OAuth{
		PublicBaseURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL")), "/"),
		GeminiClientID:      strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID")),
		CodexClientID:       strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID")),
		ClaudeClientID:      strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_CLAUDE_CLIENT_ID")),
		GitHubClientID:      strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_GITHUB_CLIENT_ID")),
		AntigravityClientID: strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_ANTIGRAVITY_CLIENT_ID")),
	}
	if cfg.CodexClientID == "default" || cfg.CodexClientID == "true" {
		cfg.CodexClientID = DefaultCodexClientID
	}
	if cfg.ClaudeClientID == "default" || cfg.ClaudeClientID == "true" {
		cfg.ClaudeClientID = DefaultClaudeClientID
	}
	if cfg.GitHubClientID == "default" || cfg.GitHubClientID == "true" {
		cfg.GitHubClientID = DefaultGitHubClientID
	}
	if cfg.AntigravityClientID == "default" || cfg.AntigravityClientID == "true" {
		cfg.AntigravityClientID = DefaultAntigravityClientID
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
			profile.DeviceAuthorizationURL = strings.TrimSpace(profile.DeviceAuthorizationURL)
			if profile.Kind != "generic" && profile.Kind != "gemini" && profile.Kind != "codex" && profile.Kind != "chatgpt" && profile.Kind != "claude" && profile.Kind != "github" && profile.Kind != "copilot" && profile.Kind != "antigravity" && profile.Kind != "agy" {
				return OAuth{}, false, fmt.Errorf("OAuth profile %d has unsupported kind %q", i, profile.Kind)
			}
			if profile.Kind == "codex" || profile.Kind == "chatgpt" {
				if profile.ClientID == "" || profile.ClientID == "default" || profile.ClientID == "public" {
					profile.ClientID = DefaultCodexClientID
				}
				if profile.ProviderID == "" {
					profile.ProviderID = "openai"
				}
			}
			if profile.Kind == "claude" {
				if profile.ClientID == "" || profile.ClientID == "default" || profile.ClientID == "public" {
					profile.ClientID = DefaultClaudeClientID
				}
				if profile.ProviderID == "" {
					profile.ProviderID = "anthropic"
				}
			}
			if profile.Kind == "github" || profile.Kind == "copilot" {
				if profile.ClientID == "" || profile.ClientID == "default" || profile.ClientID == "public" {
					profile.ClientID = DefaultGitHubClientID
				}
				if profile.ProviderID == "" {
					profile.ProviderID = "github"
				}
			}
			if profile.Kind == "antigravity" || profile.Kind == "agy" {
				if profile.ClientID == "" || profile.ClientID == "default" || profile.ClientID == "public" {
					profile.ClientID = DefaultAntigravityClientID
				}
				if profile.ProviderID == "" {
					profile.ProviderID = "antigravity"
				}
			}
			if profile.ProviderID == "" || profile.ClientID == "" {
				return OAuth{}, false, fmt.Errorf("OAuth profile %d requires provider_id and client_id", i)
			}
			if profile.Kind == "generic" && (profile.AuthorizationURL == "" || profile.TokenURL == "") {
				return OAuth{}, false, fmt.Errorf("OAuth profile %q requires authorization_url and token_url", profile.ProviderID)
			}
		}
	}

	hasClient := cfg.GeminiClientID != "" || cfg.CodexClientID != "" || cfg.ClaudeClientID != "" || cfg.GitHubClientID != "" || cfg.AntigravityClientID != "" || len(cfg.Profiles) > 0
	if cfg.PublicBaseURL == "" && !hasClient {
		return OAuth{}, false, nil
	}
	if cfg.PublicBaseURL == "" {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL is required when OAuth is enabled")
	}
	if !hasClient {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID, KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID, KOKEKOKKOR_OAUTH_CLAUDE_CLIENT_ID, KOKEKOKKOR_OAUTH_GITHUB_CLIENT_ID, KOKEKOKKOR_OAUTH_ANTIGRAVITY_CLIENT_ID, or KOKEKOKKOR_OAUTH_PROFILES_JSON is required when OAuth is enabled")
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
