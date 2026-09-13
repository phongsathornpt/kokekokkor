package secretbox

import (
	"bytes"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
)

func TestKeyringSealOpen(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	ring, err := NewKeyring("v1", map[string][]byte{"v1": key})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	ref := credential.Ref{ProviderID: "provider-a", Kind: credential.KindAPIKey}
	encrypted, err := ring.Seal(ref, []byte("value"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if bytes.Equal(encrypted.Ciphertext, []byte("value")) {
		t.Fatal("ciphertext contains plaintext")
	}
	plaintext, err := ring.Open(ref, encrypted)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if string(plaintext) != "value" {
		t.Fatalf("Open() = %q", plaintext)
	}
}

func TestKeyringBindsCiphertextToCredentialContext(t *testing.T) {
	key := bytes.Repeat([]byte{2}, 32)
	ring, err := NewKeyring("v1", map[string][]byte{"v1": key})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	encrypted, err := ring.Seal(credential.Ref{ProviderID: "provider-a", Kind: credential.KindAPIKey}, []byte("value"))
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	_, err = ring.Open(credential.Ref{ProviderID: "provider-b", Kind: credential.KindAPIKey}, encrypted)
	if err == nil {
		t.Fatal("Open() error = nil, want authentication failure")
	}
}

func TestKeyringReadsOldVersionAndWritesActiveVersion(t *testing.T) {
	keys := map[string][]byte{
		"v1": bytes.Repeat([]byte{3}, 32),
		"v2": bytes.Repeat([]byte{4}, 32),
	}
	oldRing, err := NewKeyring("v1", keys)
	if err != nil {
		t.Fatalf("NewKeyring(v1) error = %v", err)
	}
	ref := credential.Ref{ProviderID: "provider-a", Kind: credential.KindOAuthRefreshToken}
	oldEncrypted, err := oldRing.Seal(ref, []byte("refresh"))
	if err != nil {
		t.Fatalf("Seal(v1) error = %v", err)
	}
	newRing, err := NewKeyring("v2", keys)
	if err != nil {
		t.Fatalf("NewKeyring(v2) error = %v", err)
	}
	if _, err := newRing.Open(ref, oldEncrypted); err != nil {
		t.Fatalf("Open(v1 with v2 ring) error = %v", err)
	}
	newEncrypted, err := newRing.Seal(ref, []byte("refresh"))
	if err != nil {
		t.Fatalf("Seal(v2) error = %v", err)
	}
	if newEncrypted.KeyVersion != "v2" {
		t.Fatalf("KeyVersion = %q, want v2", newEncrypted.KeyVersion)
	}
}
