package config

import "testing"

func TestLoadAllowsModelRouteToAnthropicProvider(t *testing.T) {
	t.Setenv("KOKEKOKKOR_PROVIDERS_JSON", `[{"id":"openai","base_url":"https://api.openai.example","api_key":"o"}]`)
	t.Setenv("KOKEKOKKOR_DEFAULT_PROVIDER_ID", "openai")
	t.Setenv("KOKEKOKKOR_ANTHROPIC_PROVIDER_ID", "anthropic")
	t.Setenv("KOKEKOKKOR_ANTHROPIC_BASE_URL", "https://api.anthropic.example")
	t.Setenv("KOKEKOKKOR_ANTHROPIC_API_KEY", "a")
	t.Setenv("KOKEKOKKOR_MODEL_ROUTES_JSON", `{"portable":{"provider":"anthropic","model":"claude-upstream"}}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	route := cfg.ModelRoutes["portable"]
	if len(route) != 1 || route[0].ProviderID != "anthropic" || route[0].Model != "claude-upstream" {
		t.Fatalf("route = %#v", route)
	}
}

func TestValidateRejectsProtocolProviderIDCollision(t *testing.T) {
	cfg := Config{
		HTTP: HTTP{Addr: ":8080"},
		Providers: []OpenAICompatible{{
			ID:      "shared",
			BaseURL: "https://openai.example.com",
		}},
		Anthropic: Anthropic{
			ID:      "shared",
			BaseURL: "https://anthropic.example.com",
			Version: "2023-06-01",
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want duplicate provider ID error")
	}
}
