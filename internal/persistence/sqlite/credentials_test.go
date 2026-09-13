package sqlite

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	appcredentials "github.com/phongsathornpt/kokekokkor/internal/application/credentials"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
)

func TestCredentialRepositoryEncryptsAtRest(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "credentials.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	if err := store.Replace(ctx, domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "provider-a", Protocol: provider.ProtocolOpenAI, BaseURL: "https://example.com", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolOpenAI: "provider-a"},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	ring, err := secretbox.NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{7}, 32)})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	repository, err := store.Credentials(ctx, ring)
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}
	ref := credential.Ref{ProviderID: "provider-a", Kind: credential.KindAPIKey}
	if err := repository.Put(ctx, ref, []byte("provider-value")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := repository.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != "provider-value" {
		t.Fatalf("Get() = %q", got)
	}

	var ciphertext []byte
	var keyVersion string
	if err := store.db.QueryRowContext(ctx, `SELECT key_version, ciphertext FROM credentials WHERE provider_id = ? AND kind = ?`, "provider-a", string(credential.KindAPIKey)).Scan(&keyVersion, &ciphertext); err != nil {
		t.Fatalf("query ciphertext: %v", err)
	}
	if keyVersion != "v1" {
		t.Fatalf("key version = %q", keyVersion)
	}
	if bytes.Contains(ciphertext, []byte("provider-value")) {
		t.Fatal("ciphertext contains plaintext")
	}
}

func TestCredentialRepositorySupportsKeyRotation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "credentials.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	if err := store.Replace(ctx, domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "provider-a", Protocol: provider.ProtocolAnthropic, BaseURL: "https://example.com", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolAnthropic: "provider-a"},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	keys := map[string][]byte{
		"v1": bytes.Repeat([]byte{8}, 32),
		"v2": bytes.Repeat([]byte{9}, 32),
	}
	oldRing, _ := secretbox.NewKeyring("v1", keys)
	oldRepository, err := store.Credentials(ctx, oldRing)
	if err != nil {
		t.Fatalf("Credentials(v1) error = %v", err)
	}
	ref := credential.Ref{ProviderID: "provider-a", Kind: credential.KindOAuthRefreshToken}
	if err := oldRepository.Put(ctx, ref, []byte("refresh-value")); err != nil {
		t.Fatalf("Put(v1) error = %v", err)
	}

	newRing, _ := secretbox.NewKeyring("v2", keys)
	newRepository, err := store.Credentials(ctx, newRing)
	if err != nil {
		t.Fatalf("Credentials(v2) error = %v", err)
	}
	got, err := newRepository.Get(ctx, ref)
	if err != nil || string(got) != "refresh-value" {
		t.Fatalf("Get(v1 with v2 keyring) = %q, %v", got, err)
	}
	if err := newRepository.Put(ctx, ref, got); err != nil {
		t.Fatalf("Put(v2) error = %v", err)
	}
	var version string
	if err := store.db.QueryRowContext(ctx, `SELECT key_version FROM credentials WHERE provider_id = ? AND kind = ?`, ref.ProviderID, string(ref.Kind)).Scan(&version); err != nil {
		t.Fatalf("query key version: %v", err)
	}
	if version != "v2" {
		t.Fatalf("key version = %q, want v2", version)
	}
}

func TestCredentialRepositoryMissing(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "credentials.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	ring, _ := secretbox.NewKeyring("v1", map[string][]byte{"v1": bytes.Repeat([]byte{10}, 32)})
	repository, err := store.Credentials(ctx, ring)
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}
	_, err = repository.Get(ctx, credential.Ref{ProviderID: "missing", Kind: credential.KindAPIKey})
	if !errors.Is(err, appcredentials.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}
