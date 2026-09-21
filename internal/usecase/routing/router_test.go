package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestTableResolveAliasAndFallbackOrder(t *testing.T) {
	router, err := NewTable([]provider.Target{
		{ID: "primary", BaseURL: "https://primary.example.com"},
		{ID: "backup", BaseURL: "https://backup.example.com"},
	}, "primary", map[string][]RouteTarget{
		"smart": {
			{ProviderID: "primary", Model: "gpt-primary"},
			{ProviderID: "backup", Model: "gpt-backup"},
		},
	})
	if err != nil {
		t.Fatalf("NewTable() error = %v", err)
	}

	plan, err := router.Resolve(context.Background(), Request{Protocol: "openai", Model: "smart"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if plan.RequestedModel != "smart" {
		t.Fatalf("RequestedModel = %q, want smart", plan.RequestedModel)
	}
	if len(plan.Attempts) != 2 {
		t.Fatalf("len(Attempts) = %d, want 2", len(plan.Attempts))
	}
	if plan.Attempts[0].Target.ID != "primary" || plan.Attempts[0].Model != "gpt-primary" {
		t.Fatalf("first attempt = %#v", plan.Attempts[0])
	}
	if plan.Attempts[1].Target.ID != "backup" || plan.Attempts[1].Model != "gpt-backup" {
		t.Fatalf("second attempt = %#v", plan.Attempts[1])
	}
}

func TestTableRouteTargetDefaultsToRequestedModel(t *testing.T) {
	router, err := NewTable([]provider.Target{
		{ID: "fast", BaseURL: "https://fast.example.com"},
	}, "", map[string][]RouteTarget{
		"gpt-fast": {{ProviderID: "fast"}},
	})
	if err != nil {
		t.Fatalf("NewTable() error = %v", err)
	}

	plan, err := router.Resolve(context.Background(), Request{Model: "gpt-fast"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(plan.Attempts) != 1 || plan.Attempts[0].Model != "gpt-fast" {
		t.Fatalf("Resolve() = %#v", plan)
	}
}

func TestTableResolveDefaultProvider(t *testing.T) {
	router, err := NewTable([]provider.Target{
		{ID: "primary", BaseURL: "https://primary.example.com"},
	}, "primary", nil)
	if err != nil {
		t.Fatalf("NewTable() error = %v", err)
	}

	plan, err := router.Resolve(context.Background(), Request{Model: "unknown-model"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(plan.Attempts) != 1 || plan.Attempts[0].Target.ID != "primary" || plan.Attempts[0].Model != "unknown-model" {
		t.Fatalf("Resolve() = %#v", plan)
	}
	if !router.Ready() {
		t.Fatal("Ready() = false, want true")
	}
}

func TestTableNoFallbackForUnmatchedModel(t *testing.T) {
	router, err := NewTable([]provider.Target{
		{ID: "fast", BaseURL: "https://fast.example.com"},
	}, "", map[string][]RouteTarget{
		"gpt-fast": {{ProviderID: "fast"}},
	})
	if err != nil {
		t.Fatalf("NewTable() error = %v", err)
	}

	_, err = router.Resolve(context.Background(), Request{Model: "unknown-model"})
	if !errors.Is(err, ErrNoRoute) {
		t.Fatalf("Resolve() error = %v, want ErrNoRoute", err)
	}
	if !router.Ready() {
		t.Fatal("Ready() = false, want true because an exact route exists")
	}
}

func TestTableRejectsUnknownProviderAndEmptyRoute(t *testing.T) {
	t.Run("unknown provider", func(t *testing.T) {
		_, err := NewTable([]provider.Target{
			{ID: "primary", BaseURL: "https://primary.example.com"},
		}, "primary", map[string][]RouteTarget{
			"smart": {{ProviderID: "missing"}},
		})
		if !errors.Is(err, ErrUnknownProvider) {
			t.Fatalf("NewTable() error = %v, want ErrUnknownProvider", err)
		}
	})

	t.Run("empty route", func(t *testing.T) {
		_, err := NewTable([]provider.Target{
			{ID: "primary", BaseURL: "https://primary.example.com"},
		}, "primary", map[string][]RouteTarget{"smart": {}})
		if !errors.Is(err, ErrInvalidRoute) {
			t.Fatalf("NewTable() error = %v, want ErrInvalidRoute", err)
		}
	})
}

func TestTableReplacePublishesCompleteSnapshot(t *testing.T) {
	router, err := NewTable([]provider.Target{
		{ID: "primary", BaseURL: "https://primary.example.com"},
	}, "primary", nil)
	if err != nil {
		t.Fatalf("NewTable() error = %v", err)
	}

	if err := router.Replace([]provider.Target{
		{ID: "secondary", BaseURL: "https://secondary.example.com"},
	}, "secondary", map[string][]RouteTarget{
		"smart": {{ProviderID: "secondary", Model: "upstream-smart"}},
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	plan, err := router.Resolve(context.Background(), Request{Model: "smart"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(plan.Attempts) != 1 || plan.Attempts[0].Target.ID != "secondary" || plan.Attempts[0].Model != "upstream-smart" {
		t.Fatalf("Resolve() = %#v, want secondary snapshot", plan)
	}
}
