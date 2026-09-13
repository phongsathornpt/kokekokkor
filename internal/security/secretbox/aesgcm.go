package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
)

type Encrypted struct {
	KeyVersion string
	Nonce      []byte
	Ciphertext []byte
}

type Keyring struct {
	active string
	keys   map[string]cipher.AEAD
}

func NewKeyring(active string, keys map[string][]byte) (*Keyring, error) {
	if active == "" {
		return nil, fmt.Errorf("active credential key version must not be empty")
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("credential keyring must not be empty")
	}
	ring := &Keyring{active: active, keys: make(map[string]cipher.AEAD, len(keys))}
	for version, key := range keys {
		if version == "" {
			return nil, fmt.Errorf("credential key version must not be empty")
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("credential key %q must be 32 bytes", version)
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("create credential cipher %q: %w", version, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("create credential AEAD %q: %w", version, err)
		}
		ring.keys[version] = aead
	}
	if _, ok := ring.keys[active]; !ok {
		return nil, fmt.Errorf("active credential key version %q is not configured", active)
	}
	return ring, nil
}

func (k *Keyring) Seal(ref credential.Ref, plaintext []byte) (Encrypted, error) {
	if err := ref.Validate(); err != nil {
		return Encrypted{}, err
	}
	aead := k.keys[k.active]
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Encrypted{}, fmt.Errorf("generate credential nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, aad(ref, k.active))
	return Encrypted{KeyVersion: k.active, Nonce: nonce, Ciphertext: ciphertext}, nil
}

func (k *Keyring) Open(ref credential.Ref, encrypted Encrypted) ([]byte, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	aead, ok := k.keys[encrypted.KeyVersion]
	if !ok {
		return nil, fmt.Errorf("credential key version %q is not configured", encrypted.KeyVersion)
	}
	plaintext, err := aead.Open(nil, encrypted.Nonce, encrypted.Ciphertext, aad(ref, encrypted.KeyVersion))
	if err != nil {
		return nil, fmt.Errorf("decrypt credential: %w", err)
	}
	return plaintext, nil
}

func aad(ref credential.Ref, keyVersion string) []byte {
	return []byte(ref.ProviderID + "\x00" + string(ref.Kind) + "\x00" + keyVersion)
}
