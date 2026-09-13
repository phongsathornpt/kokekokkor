package config

import "testing"

func TestLoadProvidersAndLegacyModelRoute(t *testing.T) {
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
	route := cfg.ModelRoutes["gpt-fast"]
	if len(route) != 1 || route[0].ProviderID != "fast" || route[0].Model != "" {
		t.Fatalf("ModelRoutes[gpt-fast] = %#v", route)
	}
}

func TestLoadAliasAndFallbackModelRoute(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("KOKEKOKKOR_PROVIDERS_JSON", `[{"id":"primary","base_url":"https://api.example.com"},{"id":"backup","base_url":"https://backup.example.com"}]`)
	t.Setenv("KOKEKOKKOR_MODEL_ROUTES_JSON", `{"smart":[{"provider":"primary","model":"gpt-primary"},{"provider":"backup","model":"gpt-backup"}],"direct":{"provider":"primary","model":"gpt-direct"}}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	smart := cfg.ModelRoutes["smart"]
	if len(smart) != 2 {
		t.Fatalf("len(ModelRoutes[smart]) = %d, want 2", len(smart))
	}
	if smart[0].ProviderID != "primary" || smart[0].Model != "gpt-primary" {
		t.Fatalf("smart[0] = %#v", smart[0])
	}
	if smart[1].ProviderID != "backup" || smart[1].Model != "gpt-backup" {
		t.Fatalf("smart[1] = %#v", smart[1])
	}

	direct := cfg.ModelRoutes["direct"]
	if len(direct) != 1 || direct[0].ProviderID != "primary" || direct[0].Model != "gpt-direct" {
		t.Fatalf("ModelRoutes[direct] = %#v", direct)
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
	t.Setenv("KOKEKOKKOR_MODEL_ROUTES_JSON", `{"smart":[{"provider":"missing","model":"x"}]}`)

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func TestLoadRejectsEmptyFallbackRoute(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("KOKEKOKKOR_PROVIDERS_JSON", `[{"id":"primary","base_url":"https://api.example.com"}]`)
	t.Setenv("KOKEKOKKOR_MODEL_ROUTES_JSON", `{"smart":[]}`)

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
