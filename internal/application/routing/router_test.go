package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestStaticResolve(t *testing.T) {
	target := &provider.Target{ID: "primary", BaseURL: "https://example.com"}
	router := NewStatic(target)

	got, err := router.Resolve(context.Background(), Request{Protocol: "openai"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.ID != target.ID || got.BaseURL != target.BaseURL {
		t.Fatalf("Resolve() = %#v, want %#v", got, *target)
	}
	if !router.Ready() {
		t.Fatal("Ready() = false, want true")
	}
}

func TestStaticNoRoute(t *testing.T) {
	router := NewStatic(nil)
	_, err := router.Resolve(context.Background(), Request{})
	if !errors.Is(err, ErrNoRoute) {
		t.Fatalf("Resolve() error = %v, want ErrNoRoute", err)
	}
	if router.Ready() {
		t.Fatal("Ready() = true, want false")
	}
}
