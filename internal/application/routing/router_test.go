package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestTableResolveModelRouteAndDefault(t *testing.T) {
	router, err := NewTable([]provider.Target{
		{ID: "primary", BaseURL: "https://primary.example.com"},
		{ID: "fast", BaseURL: "https://fast.example.com"},
	}, "primary", map[string]string{
		"gpt-fast": "fast",
	})
	if err != nil {
		t.Fatalf("NewTable() error = %v", err)
	}

	got, err := router.Resolve(context.Background(), Request{Protocol: "openai", Model: "gpt-fast"})
	if err != nil {
		t.Fatalf("Resolve(model route) error = %v", err)
	}
	if got.ID != "fast" {
		t.Fatalf("Resolve(model route).ID = %q, want %q", got.ID, "fast")
	}

	got, err = router.Resolve(context.Background(), Request{Protocol: "openai", Model: "unknown-model"})
	if err != nil {
		t.Fatalf("Resolve(default route) error = %v", err)
	}
	if got.ID != "primary" {
		t.Fatalf("Resolve(default route).ID = %q, want %q", got.ID, "primary")
	}
	if !router.Ready() {
		t.Fatal("Ready() = false, want true")
	}
}

func TestTableNoFallbackForUnmatchedModel(t *testing.T) {
	router, err := NewTable([]provider.Target{
		{ID: "fast", BaseURL: "https://fast.example.com"},
	}, "", map[string]string{
		"gpt-fast": "fast",
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

func TestTableRejectsUnknownProvider(t *testing.T) {
	_, err := NewTable([]provider.Target{
		{ID: "primary", BaseURL: "https://primary.example.com"},
	}, "primary", map[string]string{
		"gpt-fast": "missing",
	})
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("NewTable() error = %v, want ErrUnknownProvider", err)
	}
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
	}, "secondary", map[string]string{"gpt-secondary": "secondary"}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	got, err := router.Resolve(context.Background(), Request{Model: "gpt-secondary"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.ID != "secondary" || got.BaseURL != "https://secondary.example.com" {
		t.Fatalf("Resolve() = %#v, want secondary snapshot", got)
	}
}
