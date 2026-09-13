package bootstrap

import (
	"context"
	"fmt"
	"net/http"

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/persistence/sqlite"
	provideroauth "github.com/phongsathornpt/kokekokkor/internal/provider/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
	"github.com/phongsathornpt/kokekokkor/internal/transport/oauthhttp"
)

func resolveOAuthHandler(ctx context.Context, cfg config.Config, store *sqlitestore.Store) (http.Handler, error) {
	oauthConfig, enabled, err := config.LoadOAuth()
	if err != nil || !enabled {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("OAuth requires KOKEKOKKOR_DATABASE_DSN")
	}

	encryption, encryptionEnabled, err := config.LoadCredentialEncryption()
	if err != nil {
		return nil, err
	}
	if !encryptionEnabled {
		return nil, fmt.Errorf("OAuth requires encrypted credential storage")
	}
	keyring, err := secretbox.NewKeyring(encryption.ActiveKeyVersion, encryption.Keys)
	if err != nil {
		return nil, err
	}
	credentials, err := store.Credentials(ctx, keyring)
	if err != nil {
		return nil, err
	}
	tokens := appoauth.NewCredentialTokenRepository(credentials)
	service := appoauth.NewService(appoauth.NewMemoryStateRepository(), provideroauth.NewExchanger(http.DefaultClient), tokens)

	profile, err := provideroauth.GeminiProfile(provideroauth.ProfileOptions{
		ProviderID: cfg.Gemini.ID,
		ClientID:   oauthConfig.GeminiClientID,
	})
	if err != nil {
		return nil, err
	}
	return oauthhttp.New(service, map[string]oauth.Provider{profile.ID: profile}, oauthConfig.PublicBaseURL)
}
