package catalog

import (
	"testing"
)

func TestDefaultModelsForProviderCoversBuiltInProviders(t *testing.T) {
	providers := []struct {
		id         string
		defaultID  string
		retiredIDs []string
	}{
		{id: "openai", defaultID: "gpt-5.6-sol"},
		{id: "anthropic", defaultID: "claude-sonnet-5", retiredIDs: []string{"claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022"}},
		{id: "gemini", defaultID: "gemini-3.8-flash", retiredIDs: []string{"gemini-2.0-flash"}},
		{id: "antigravity", defaultID: "claude-opus-4-6-thinking"},
		{id: "deepseek", defaultID: "deepseek-v4-pro"},
		{id: "groq", defaultID: "openai/gpt-oss-120b"},
		{id: "openrouter", defaultID: "deepseek/deepseek-v4-pro"},
		{id: "ollama", defaultID: "deepseek-v4-flash"},
	}

	for _, tc := range providers {
		t.Run(tc.id, func(t *testing.T) {
			models := DefaultModelsForProvider(tc.id)
			if len(models) == 0 {
				t.Fatal("expected a non-empty model catalog")
			}
			defaults := 0
			foundDefault := false
			for _, model := range models {
				if model.ID == "" || model.Name == "" {
					t.Fatalf("model has incomplete metadata: %#v", model)
				}
				if model.IsDefault {
					defaults++
					foundDefault = model.ID == tc.defaultID
				}
				for _, retiredID := range tc.retiredIDs {
					if model.ID == retiredID {
						t.Fatalf("retired model %q remains in catalog", retiredID)
					}
				}
			}
			if defaults != 1 || !foundDefault {
				t.Fatalf("defaults=%d foundDefault=%v, want exactly %q", defaults, foundDefault, tc.defaultID)
			}
		})
	}
}

func TestDefaultModelsForProvider(t *testing.T) {
	antigravity := DefaultModelsForProvider("antigravity")
	if len(antigravity) == 0 {
		t.Fatalf("expected Antigravity models, got none")
	}

	var foundClaudeOpus bool
	for _, m := range antigravity {
		if m.ID == "claude-opus-4-6-thinking" {
			foundClaudeOpus = true
			if !m.IsDefault {
				t.Errorf("claude-opus-4-6-thinking should be default")
			}
			if len(m.Capabilities) == 0 {
				t.Errorf("claude-opus-4-6-thinking should have capabilities")
			}
		}
	}
	if !foundClaudeOpus {
		t.Errorf("expected claude-opus-4-6-thinking in Antigravity models")
	}

	agy := DefaultModelsForProvider("AGY")
	if len(agy) != len(antigravity) {
		t.Errorf("AGY alias should return Antigravity models, got %d", len(agy))
	}

	openai := DefaultModelsForProvider("openai")
	if len(openai) == 0 {
		t.Fatalf("expected OpenAI models, got none")
	}

	unknown := DefaultModelsForProvider("unknown-provider-xyz")
	if len(unknown) != 0 {
		t.Errorf("expected nil for unknown provider, got %d", len(unknown))
	}
}

func TestDefaultModelsForProviderDoesNotInferCustomIDs(t *testing.T) {
	if models := DefaultModelsForProvider("my-openai-compatible"); len(models) != 0 {
		t.Fatalf("custom provider unexpectedly received a static catalog: %#v", models)
	}
}

func TestModelEffectiveUpstreamModel(t *testing.T) {
	m1 := Model{ID: "test-model"}
	if m1.EffectiveUpstreamModel() != "test-model" {
		t.Errorf("got %q, want test-model", m1.EffectiveUpstreamModel())
	}

	m2 := Model{ID: "test-model", UpstreamModel: "upstream-123(high)"}
	if m2.EffectiveUpstreamModel() != "upstream-123(high)" {
		t.Errorf("got %q, want upstream-123(high)", m2.EffectiveUpstreamModel())
	}
}
