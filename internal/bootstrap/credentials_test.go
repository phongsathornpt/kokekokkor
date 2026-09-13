package bootstrap

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/persistence/sqlite"
)

func TestResolveCredentialsSeedsOnceAndPrefersPersistedValue(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", `{"v1":"`+key+`"}`)
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")

	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "credentials.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	snapshot := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "provider-a", Protocol: provider.ProtocolOpenAI, BaseURL: "https://example.com", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolOpenAI: "provider-a"},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}
	if err := store.Replace(ctx, snapshot); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	cfg := config.Config{Providers: []config.OpenAICompatible{{ID: "provider-a", BaseURL: "https://example.com", APIKey: "first-value"}}}
	resolved, err := resolveCredentials(ctx, cfg, snapshot, store)
	if err != nil {
		t.Fatalf("resolveCredentials(first) error = %v", err)
	}
	if resolved["provider-a"] != "first-value" {
		t.Fatalf("first resolved value = %q", resolved["provider-a"])
	}

	cfg.Providers[0].APIKey = "second-value"
	resolved, err = resolveCredentials(ctx, cfg, snapshot, store)
	if err != nil {
		t.Fatalf("resolveCredentials(second) error = %v", err)
	}
	if resolved["provider-a"] != "first-value" {
		t.Fatalf("persisted value = %q, want first-value", resolved["provider-a"])
	}
}
