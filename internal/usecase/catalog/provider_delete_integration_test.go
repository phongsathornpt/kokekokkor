package catalog

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/repository/sqlite"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
)

func TestDeleteProviderPreservesCredentialWhenRuntimeApplyFails(t *testing.T) {
	ctx := context.Background()
	store, credentialRepository := providerDeleteFixture(t, ctx)
	defer store.Close()

	service := NewService(store, func(domaincatalog.Snapshot) error { return errors.New("runtime failed") })
	if err := service.DeleteProvider(ctx, "one"); err == nil {
		t.Fatal("DeleteProvider() error = nil")
	}

	value, err := credentialRepository.Get(ctx, credential.Ref{ProviderID: "one", Kind: credential.KindAPIKey})
	if err != nil || string(value) != "secret" {
		t.Fatalf("credential after failed delete = %q, %v", value, err)
	}
	persisted, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(persisted.Providers) != 1 || persisted.Providers[0].ID != "one" {
		t.Fatalf("provider after failed delete = %#v", persisted.Providers)
	}
}

func TestDeleteProviderCascadesCredentialAfterSuccessfulRuntimeApply(t *testing.T) {
	ctx := context.Background()
	store, credentialRepository := providerDeleteFixture(t, ctx)
	defer store.Close()

	service := NewService(store, func(domaincatalog.Snapshot) error { return nil })
	if err := service.DeleteProvider(ctx, "one"); err != nil {
		t.Fatalf("DeleteProvider() error = %v", err)
	}

	_, err := credentialRepository.Get(ctx, credential.Ref{ProviderID: "one", Kind: credential.KindAPIKey})
	if !errors.Is(err, appcredential.ErrNotFound) {
		t.Fatalf("credential after delete = %v, want ErrNotFound", err)
	}
	persisted, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(persisted.Providers) != 0 {
		t.Fatalf("providers after delete = %#v", persisted.Providers)
	}
}

func providerDeleteFixture(t *testing.T, ctx context.Context) (*sqlitestore.Store, *sqlitestore.CredentialRepository) {
	t.Helper()
	store, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	snapshot := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "one", Protocol: provider.ProtocolOpenAI, BaseURL: "https://one.example", Enabled: true}},
		Defaults:  map[provider.Protocol]string{},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}
	if err := store.Replace(ctx, snapshot); err != nil {
		store.Close()
		t.Fatalf("Replace() error = %v", err)
	}
	ring, err := secretbox.NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{4}, 32)})
	if err != nil {
		store.Close()
		t.Fatalf("NewKeyring() error = %v", err)
	}
	credentialRepository, err := store.Credentials(ctx, ring)
	if err != nil {
		store.Close()
		t.Fatalf("Credentials() error = %v", err)
	}
	if err := credentialRepository.Put(ctx, credential.Ref{ProviderID: "one", Kind: credential.KindAPIKey}, []byte("secret")); err != nil {
		store.Close()
		t.Fatalf("Put() error = %v", err)
	}
	return store, credentialRepository
}
