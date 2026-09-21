package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/repository/sqlite"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
)

func resolveCredentials(ctx context.Context, cfg config.Config, snapshot domaincatalog.Snapshot, store *sqlitestore.Store) (*appcredential.Service, error) {
	environment := environmentCredentials(cfg)
	encryption, enabled, err := config.LoadCredentialEncryption()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return appcredential.NewService(nil, environment), nil
	}
	if store == nil {
		return nil, fmt.Errorf("credential encryption requires KOKEKOKKOR_DATABASE_DSN")
	}
	keyring, err := secretbox.NewKeyring(encryption.ActiveKeyVersion, encryption.Keys)
	if err != nil {
		return nil, err
	}
	repository, err := store.Credentials(ctx, keyring)
	if err != nil {
		return nil, err
	}

	resolved := make(map[string]string, len(snapshot.Providers))
	for _, provider := range snapshot.Providers {
		ref := credential.Ref{ProviderID: provider.ID, Kind: credential.KindAPIKey}
		value, err := repository.Get(ctx, ref)
		switch {
		case err == nil:
			resolved[provider.ID] = string(value)
		case errors.Is(err, appcredential.ErrNotFound):
			seed := environment[provider.ID]
			if seed == "" {
				continue
			}
			if err := repository.Put(ctx, ref, []byte(seed)); err != nil {
				return nil, err
			}
			resolved[provider.ID] = seed
		default:
			return nil, err
		}
	}
	return appcredential.NewService(repository, resolved), nil
}

func environmentCredentials(cfg config.Config) map[string]string {
	credentials := make(map[string]string, len(cfg.Providers)+2)
	for _, item := range cfg.Providers {
		credentials[item.ID] = item.APIKey
	}
	if cfg.Anthropic.ID != "" {
		credentials[cfg.Anthropic.ID] = cfg.Anthropic.APIKey
	}
	if cfg.Gemini.ID != "" {
		credentials[cfg.Gemini.ID] = cfg.Gemini.APIKey
	}
	return credentials
}
