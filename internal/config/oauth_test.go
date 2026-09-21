package config

import "testing"

func TestLoadOAuthDisabled(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")
	_, enabled, err := LoadOAuth()
	if err != nil {
		t.Fatalf("LoadOAuth() error = %v", err)
	}
	if enabled {
		t.Fatal("enabled = true, want false")
	}
}

func TestLoadOAuthGemini(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com/")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "client-id")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")
	cfg, enabled, err := LoadOAuth()
	if err != nil {
		t.Fatalf("LoadOAuth() error = %v", err)
	}
	if !enabled || cfg.PublicBaseURL != "https://gateway.example.com" || cfg.GeminiClientID != "client-id" {
		t.Fatalf("cfg = %#v enabled=%v", cfg, enabled)
	}
}

func TestLoadOAuthCodexDefault(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "default")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")
	cfg, enabled, err := LoadOAuth()
	if err != nil {
		t.Fatalf("LoadOAuth() error = %v", err)
	}
	if !enabled || cfg.CodexClientID != DefaultCodexClientID {
		t.Fatalf("cfg = %#v enabled=%v", cfg, enabled)
	}
}

func TestLoadOAuthCodexCustom(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "my-custom-client-id")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")
	cfg, enabled, err := LoadOAuth()
	if err != nil {
		t.Fatalf("LoadOAuth() error = %v", err)
	}
	if !enabled || cfg.CodexClientID != "my-custom-client-id" {
		t.Fatalf("cfg = %#v enabled=%v", cfg, enabled)
	}
}

func TestLoadOAuthAllowsLoopbackHTTP(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "client-id")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")
	if _, enabled, err := LoadOAuth(); err != nil || !enabled {
		t.Fatalf("LoadOAuth() enabled=%v error=%v", enabled, err)
	}
}

func TestLoadOAuthRejectsPartialConfiguration(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CLAUDE_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_GITHUB_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")
	if _, _, err := LoadOAuth(); err == nil {
		t.Fatal("LoadOAuth() error = nil, want client ID validation error")
	}
}
