package routing

import (
	"context"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestGeminiProtocolDefault(t *testing.T) {
	router, err := NewProtocolTable([]provider.Target{
		{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://generativelanguage.googleapis.com"},
	}, map[provider.Protocol]string{
		provider.ProtocolGemini: "gemini",
	}, nil)
	if err != nil {
		t.Fatalf("NewProtocolTable() error = %v", err)
	}

	plan, err := router.Resolve(context.Background(), Request{Protocol: provider.ProtocolGemini, Model: "gemini-test"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(plan.Attempts) != 1 || plan.Attempts[0].Target.ID != "gemini" || plan.Attempts[0].Target.Protocol != provider.ProtocolGemini {
		t.Fatalf("plan = %#v", plan)
	}
}
