package bootstrap

import (
	"context"
	"fmt"
	"net/http"

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/persistence/sqlite"
	provideroauth "github.com/phongsathornpt/kokekokkor/internal/provider/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
	"github.com/phongsathornpt/kokekokkor/internal/transport/oauthhttp"
)

type oauthRuntime struct {
	handler      http.Handler
	bearerTokens *appoauth.RuntimeTokenResolver
}

func resolveOAuthRuntime(ctx context.Context, cfg config.Config, store *sqlitestore.Store) (oauthRuntime, error) {
	oauthConfig, enabled, err := config.LoadOAuth()
	if err != nil || !enabled {
		return oauthRuntime{}, err
	}
	if store == nil {
		return oauthRuntime{}, fmt.Errorf("OAuth requires KOKEKOKKOR_DATABASE_DSN")
	}

	encryption, encryptionEnabled, err := config.LoadCredentialEncryption()
	if err != nil {
		return oauthRuntime{}, err
	}
	if !encryptionEnabled {
		return oauthRuntime{}, fmt.Errorf("OAuth requires encrypted credential storage")
	}
	keyring, err := secretbox.NewKeyring(encryption.ActiveKeyVersion, encryption.Keys)
	if err != nil {
		return oauthRuntime{}, err
	}
	credentials, err := store.Credentials(ctx, keyring)
	if err != nil {
		return oauthRuntime{}, err
	}
	tokens := appoauth.NewCredentialTokenRepository(credentials)
	exchanger := provideroauth.NewExchanger(http.DefaultClient)

	profile, err := provideroauth.GeminiProfile(provideroauth.ProfileOptions{
		ProviderID: cfg.Gemini.ID,
		ClientID:   oauthConfig.GeminiClientID,
	})
	if err != nil {
		return oauthRuntime{}, err
	}
	profiles := map[string]domainoauth.Provider{profile.ID: profile}
	service := appoauth.NewService(appoauth.NewMemoryStateRepository(), exchanger, tokens)
	handler, err := oauthhttp.New(service, profiles, oauthConfig.PublicBaseURL)
	if err != nil {
		return oauthRuntime{}, err
	}
	return oauthRuntime{
		handler:      handler,
		bearerTokens: appoauth.NewRuntimeTokenResolver(tokens, exchanger, profiles),
	}, nil
}
