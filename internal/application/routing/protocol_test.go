package routing

import (
	"context"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestProtocolDefaultsStayProtocolLocal(t *testing.T) {
	router, err := NewProtocolTable([]provider.Target{
		{ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example.com"},
		{ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example.com"},
	}, map[provider.Protocol]string{
		provider.ProtocolOpenAI:    "openai",
		provider.ProtocolAnthropic: "anthropic",
	}, nil)
	if err != nil {
		t.Fatalf("NewProtocolTable() error = %v", err)
	}

	openAIPlan, err := router.Resolve(context.Background(), Request{Protocol: provider.ProtocolOpenAI, Model: "same-name"})
	if err != nil {
		t.Fatalf("Resolve(OpenAI) error = %v", err)
	}
	anthropicPlan, err := router.Resolve(context.Background(), Request{Protocol: provider.ProtocolAnthropic, Model: "same-name"})
	if err != nil {
		t.Fatalf("Resolve(Anthropic) error = %v", err)
	}
	if got := openAIPlan.Attempts[0].Target.ID; got != "openai" {
		t.Fatalf("OpenAI default = %q", got)
	}
	if got := anthropicPlan.Attempts[0].Target.ID; got != "anthropic" {
		t.Fatalf("Anthropic default = %q", got)
	}
}

func TestExactRouteCanCrossProtocols(t *testing.T) {
	router, err := NewProtocolTable([]provider.Target{
		{ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example.com"},
		{ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://anthropic.example.com"},
	}, nil, map[string][]RouteTarget{
		"portable": {{ProviderID: "anthropic", Model: "claude-upstream"}},
	})
	if err != nil {
		t.Fatalf("NewProtocolTable() error = %v", err)
	}

	plan, err := router.Resolve(context.Background(), Request{Protocol: provider.ProtocolOpenAI, Model: "portable"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(plan.Attempts) != 1 || plan.Attempts[0].Target.Protocol != provider.ProtocolAnthropic || plan.Attempts[0].Model != "claude-upstream" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestProtocolDefaultMustMatchTargetProtocol(t *testing.T) {
	_, err := NewProtocolTable([]provider.Target{
		{ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://openai.example.com"},
	}, map[provider.Protocol]string{
		provider.ProtocolAnthropic: "openai",
	}, nil)
	if err == nil {
		t.Fatal("NewProtocolTable() error = nil, want protocol mismatch")
	}
}
