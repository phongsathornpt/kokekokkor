package config

import "testing"

func TestLoadGeminiProvider(t *testing.T) {
	t.Setenv("KOKEKOKKOR_GEMINI_PROVIDER_ID", "google")
	t.Setenv("KOKEKOKKOR_GEMINI_BASE_URL", "https://generativelanguage.googleapis.com")
	t.Setenv("KOKEKOKKOR_GEMINI_API_KEY", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Gemini.ID != "google" || cfg.Gemini.BaseURL != "https://generativelanguage.googleapis.com" || cfg.Gemini.APIKey != "secret" {
		t.Fatalf("Gemini = %#v", cfg.Gemini)
	}
}

func TestLoadAllowsModelRouteToGeminiProvider(t *testing.T) {
	t.Setenv("KOKEKOKKOR_GEMINI_BASE_URL", "https://generativelanguage.googleapis.com")
	t.Setenv("KOKEKOKKOR_MODEL_ROUTES_JSON", `{"portable":{"provider":"gemini","model":"gemini-upstream"}}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	route := cfg.ModelRoutes["portable"]
	if len(route) != 1 || route[0].ProviderID != "gemini" || route[0].Model != "gemini-upstream" {
		t.Fatalf("route = %#v", route)
	}
}

func TestValidateRejectsGeminiProviderIDCollision(t *testing.T) {
	cfg := Config{
		HTTP: HTTP{Addr: ":8080"},
		Providers: []OpenAICompatible{{
			ID:      "shared",
			BaseURL: "https://openai.example.com",
		}},
		Gemini: Gemini{
			ID:      "shared",
			BaseURL: "https://generativelanguage.googleapis.com",
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want duplicate provider ID error")
	}
}
