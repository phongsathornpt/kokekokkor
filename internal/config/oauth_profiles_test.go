package config

import "testing"

func TestLoadOAuthProfiles(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", `[{"provider_id":"openai-alt","client_id":"client","authorization_url":"https://auth.example.com/authorize","token_url":"https://auth.example.com/token","scopes":["models.read"]},{"kind":"gemini","provider_id":"gemini-alt","client_id":"gemini-client"}]`)

	cfg, enabled, err := LoadOAuth()
	if err != nil {
		t.Fatalf("LoadOAuth() error = %v", err)
	}
	if !enabled || len(cfg.Profiles) != 2 {
		t.Fatalf("enabled=%v profiles=%#v", enabled, cfg.Profiles)
	}
	if cfg.Profiles[0].Kind != "generic" || cfg.Profiles[1].Kind != "gemini" {
		t.Fatalf("profiles=%#v", cfg.Profiles)
	}
}

func TestLoadOAuthProfilesRejectsIncompleteGenericProfile(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", `[{"provider_id":"p","client_id":"client"}]`)
	if _, _, err := LoadOAuth(); err == nil {
		t.Fatal("LoadOAuth() error = nil")
	}
}

func TestLoadOAuthProfilesCodexAndChatGPT(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", `[{"kind":"codex"},{"kind":"chatgpt","provider_id":"openai-custom","client_id":"custom-id"}]`)

	cfg, enabled, err := LoadOAuth()
	if err != nil {
		t.Fatalf("LoadOAuth() error = %v", err)
	}
	if !enabled || len(cfg.Profiles) != 2 {
		t.Fatalf("enabled=%v profiles=%#v", enabled, cfg.Profiles)
	}
	if cfg.Profiles[0].Kind != "codex" || cfg.Profiles[0].ProviderID != "openai" || cfg.Profiles[0].ClientID != DefaultCodexClientID {
		t.Fatalf("profile[0]=%#v", cfg.Profiles[0])
	}
	if cfg.Profiles[1].Kind != "chatgpt" || cfg.Profiles[1].ProviderID != "openai-custom" || cfg.Profiles[1].ClientID != "custom-id" {
		t.Fatalf("profile[1]=%#v", cfg.Profiles[1])
	}
}
