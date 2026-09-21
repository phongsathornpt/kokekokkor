package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
)

type CredentialRepository struct {
	db      *sql.DB
	keyring *secretbox.Keyring
}

func (s *Store) Credentials(ctx context.Context, keyring *secretbox.Keyring) (*CredentialRepository, error) {
	if keyring == nil {
		return nil, fmt.Errorf("credential keyring must not be nil")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS credentials (
			provider_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			key_version TEXT NOT NULL,
			nonce BLOB NOT NULL,
			ciphertext BLOB NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (provider_id, kind),
			FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
		)`,
		`INSERT OR IGNORE INTO schema_migrations(version) VALUES (2)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return nil, fmt.Errorf("migrate credential store: %w", err)
		}
	}
	return &CredentialRepository{db: s.db, keyring: keyring}, nil
}

func (r *CredentialRepository) Get(ctx context.Context, ref credential.Ref) ([]byte, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	var encrypted secretbox.Encrypted
	err := r.db.QueryRowContext(ctx,
		`SELECT key_version, nonce, ciphertext FROM credentials WHERE provider_id = ? AND kind = ?`,
		ref.ProviderID, string(ref.Kind),
	).Scan(&encrypted.KeyVersion, &encrypted.Nonce, &encrypted.Ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appcredential.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load credential %s/%s: %w", ref.ProviderID, ref.Kind, err)
	}
	return r.keyring.Open(ref, encrypted)
}

func (r *CredentialRepository) Put(ctx context.Context, ref credential.Ref, plaintext []byte) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	encrypted, err := r.keyring.Seal(ref, plaintext)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO credentials(provider_id, kind, key_version, nonce, ciphertext, updated_at)
		VALUES (?, ?, ?, ?, ?, unixepoch())
		ON CONFLICT(provider_id, kind) DO UPDATE SET
			key_version = excluded.key_version,
			nonce = excluded.nonce,
			ciphertext = excluded.ciphertext,
			updated_at = unixepoch()
	`, ref.ProviderID, string(ref.Kind), encrypted.KeyVersion, encrypted.Nonce, encrypted.Ciphertext)
	if err != nil {
		return fmt.Errorf("store credential %s/%s: %w", ref.ProviderID, ref.Kind, err)
	}
	return nil
}

func (r *CredentialRepository) Delete(ctx context.Context, ref credential.Ref) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM credentials WHERE provider_id = ? AND kind = ?`,
		ref.ProviderID, string(ref.Kind),
	)
	if err != nil {
		return fmt.Errorf("delete credential %s/%s: %w", ref.ProviderID, ref.Kind, err)
	}
	return nil
}
