package bootstrap

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

func TestResolveCatalogSeedsThenPrefersPersistence(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		DatabaseDSN:       filepath.Join(t.TempDir(), "catalog.db"),
		Providers:         []config.OpenAICompatible{{ID: "openai", BaseURL: "https://api.openai.com", APIKey: "secret"}},
		DefaultProviderID: "openai",
		ModelRoutes: map[string][]config.ModelRouteTarget{
			"portable": {{ProviderID: "openai", Model: "gpt-upstream"}},
		},
		Anthropic: config.Anthropic{ID: "anthropic", Version: "2023-06-01"},
		Gemini:    config.Gemini{ID: "gemini"},
	}

	first, store, err := resolveCatalog(ctx, cfg)
	if err != nil {
		t.Fatalf("resolveCatalog(first) error = %v", err)
	}
	if store == nil {
		t.Fatal("resolveCatalog(first) store = nil")
	}
	if got := first.Defaults[provider.ProtocolOpenAI]; got != "openai" {
		t.Fatalf("seeded OpenAI default = %q", got)
	}

	persisted := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "persisted", Protocol: provider.ProtocolOpenAI, BaseURL: "https://persisted.example", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolOpenAI: "persisted"},
		Routes:    map[string][]domaincatalog.RouteTarget{"portable": {{ProviderID: "persisted", Model: "persisted-model"}}},
	}
	if err := store.Replace(ctx, persisted); err != nil {
		t.Fatalf("Replace(persisted) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close(first store) error = %v", err)
	}

	second, secondStore, err := resolveCatalog(ctx, cfg)
	if err != nil {
		t.Fatalf("resolveCatalog(second) error = %v", err)
	}
	defer secondStore.Close()
	if !reflect.DeepEqual(second, persisted) {
		t.Fatalf("second snapshot = %#v, want %#v", second, persisted)
	}
}

func TestRoutingInputsKeepSecretsOutOfCatalog(t *testing.T) {
	snapshot := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://persisted.example", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolOpenAI: "openai"},
		Routes:    map[string][]domaincatalog.RouteTarget{"portable": {{ProviderID: "openai", Model: "gpt-upstream"}}},
	}
	credentials := map[string]string{"openai": "resolved-secret"}
	targets, defaults, routes := routingInputs(snapshot, credentials)
	if len(targets) != 1 {
		t.Fatalf("targets = %#v", targets)
	}
	if targets[0].BaseURL != "https://persisted.example" || targets[0].APIKey != "resolved-secret" {
		t.Fatalf("target = %#v", targets[0])
	}
	if defaults[provider.ProtocolOpenAI] != "openai" || routes["portable"][0].Model != "gpt-upstream" {
		t.Fatalf("defaults=%#v routes=%#v", defaults, routes)
	}
	if snapshot.Providers[0].BaseURL != "https://persisted.example" {
		t.Fatalf("snapshot mutated: %#v", snapshot)
	}
}
