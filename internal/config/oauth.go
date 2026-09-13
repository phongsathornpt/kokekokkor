package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type OAuth struct {
	PublicBaseURL  string
	GeminiClientID string
}

func LoadOAuth() (OAuth, bool, error) {
	cfg := OAuth{
		PublicBaseURL:  strings.TrimRight(strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL")), "/"),
		GeminiClientID: strings.TrimSpace(os.Getenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID")),
	}
	if cfg.PublicBaseURL == "" && cfg.GeminiClientID == "" {
		return OAuth{}, false, nil
	}
	if cfg.PublicBaseURL == "" {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL is required when OAuth is enabled")
	}
	if cfg.GeminiClientID == "" {
		return OAuth{}, false, fmt.Errorf("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID is required when OAuth is enabled")
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
