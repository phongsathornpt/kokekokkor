package catalog

import (
	"testing"
)

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
