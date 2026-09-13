package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestStoreReplaceAndLoad(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	want := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{
			{ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://api.anthropic.com", Enabled: true},
			{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://generativelanguage.googleapis.com", Enabled: true},
		},
		Routes: map[string][]domaincatalog.RouteTarget{
			"portable": {
				{ProviderID: "gemini", Model: "gemini-upstream"},
				{ProviderID: "anthropic", Model: "claude-upstream"},
			},
		},
	}
	if err := store.Replace(ctx, want); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestStoreReplaceIsAtomicOnValidationError(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	initial := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://generativelanguage.googleapis.com", Enabled: true}},
		Routes: map[string][]domaincatalog.RouteTarget{"portable": {{ProviderID: "gemini"}}},
	}
	if err := store.Replace(ctx, initial); err != nil {
		t.Fatalf("Replace(initial) error = %v", err)
	}
	invalid := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://generativelanguage.googleapis.com", Enabled: true}},
		Routes: map[string][]domaincatalog.RouteTarget{"portable": {{ProviderID: "missing"}}},
	}
	if err := store.Replace(ctx, invalid); err == nil {
		t.Fatal("Replace(invalid) error = nil, want validation error")
	}
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got, initial) {
		t.Fatalf("snapshot changed after failed replace: %#v", got)
	}
}

func TestStoreMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "catalog.db")
	first, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	second, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer second.Close()
	if _, err := second.Load(ctx); err != nil {
		t.Fatalf("Load() after repeated migration error = %v", err)
	}
}
