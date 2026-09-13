package config

import "testing"

func TestLoadAnthropicProvider(t *testing.T) {
	t.Setenv("KOKEKOKKOR_ANTHROPIC_PROVIDER_ID", "claude")
	t.Setenv("KOKEKOKKOR_ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	t.Setenv("KOKEKOKKOR_ANTHROPIC_API_KEY", "secret")
	t.Setenv("KOKEKOKKOR_ANTHROPIC_VERSION", "2023-06-01")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Anthropic.ID != "claude" || cfg.Anthropic.BaseURL != "https://api.anthropic.com" || cfg.Anthropic.APIKey != "secret" {
		t.Fatalf("Anthropic = %#v", cfg.Anthropic)
	}
	if cfg.Anthropic.Version != "2023-06-01" {
		t.Fatalf("Anthropic.Version = %q", cfg.Anthropic.Version)
	}
}

func TestLoadAnthropicDefaultsVersion(t *testing.T) {
	t.Setenv("KOKEKOKKOR_ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	t.Setenv("KOKEKOKKOR_ANTHROPIC_VERSION", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Anthropic.Version != "2023-06-01" {
		t.Fatalf("Anthropic.Version = %q, want default", cfg.Anthropic.Version)
	}
}
