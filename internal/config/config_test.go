package config

import "testing"

func TestLoadProvidersAndModelRoutes(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("KOKEKOKKOR_PROVIDERS_JSON", `[{"id":"primary","base_url":"https://api.example.com","api_key":"one"},{"id":"fast","base_url":"https://fast.example.com","api_key":"two"}]`)
	t.Setenv("KOKEKOKKOR_DEFAULT_PROVIDER_ID", "primary")
	t.Setenv("KOKEKOKKOR_MODEL_ROUTES_JSON", `{"gpt-fast":"fast"}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(cfg.Providers))
	}
	if cfg.DefaultProviderID != "primary" {
		t.Fatalf("DefaultProviderID = %q, want %q", cfg.DefaultProviderID, "primary")
	}
	if got := cfg.ModelRoutes["gpt-fast"]; got != "fast" {
		t.Fatalf("ModelRoutes[gpt-fast] = %q, want %q", got, "fast")
	}
}

func TestLoadLegacySingleProvider(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("KOKEKOKKOR_OPENAI_PROVIDER_ID", "legacy")
	t.Setenv("KOKEKOKKOR_OPENAI_BASE_URL", "https://legacy.example.com")
	t.Setenv("KOKEKOKKOR_OPENAI_API_KEY", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("len(Providers) = %d, want 1", len(cfg.Providers))
	}
	if cfg.Providers[0].ID != "legacy" || cfg.Providers[0].APIKey != "secret" {
		t.Fatalf("Providers[0] = %#v, want legacy provider", cfg.Providers[0])
	}
	if cfg.DefaultProviderID != "legacy" {
		t.Fatalf("DefaultProviderID = %q, want %q", cfg.DefaultProviderID, "legacy")
	}
}

func TestLoadRejectsUnknownRouteProvider(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("KOKEKOKKOR_PROVIDERS_JSON", `[{"id":"primary","base_url":"https://api.example.com"}]`)
	t.Setenv("KOKEKOKKOR_MODEL_ROUTES_JSON", `{"gpt-fast":"missing"}`)

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"KOKEKOKKOR_PROVIDERS_JSON",
		"KOKEKOKKOR_DEFAULT_PROVIDER_ID",
		"KOKEKOKKOR_MODEL_ROUTES_JSON",
		"KOKEKOKKOR_OPENAI_PROVIDER_ID",
		"KOKEKOKKOR_OPENAI_BASE_URL",
		"KOKEKOKKOR_OPENAI_API_KEY",
	} {
		t.Setenv(key, "")
	}
}
