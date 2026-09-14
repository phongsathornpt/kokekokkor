package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/persistence/sqlite"
	provideroauth "github.com/phongsathornpt/kokekokkor/internal/provider/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
	"github.com/phongsathornpt/kokekokkor/internal/transport/oauthhttp"
)

type oauthRuntime struct {
	handler      http.Handler
	bearerTokens *appoauth.RuntimeTokenResolver
	tokens       *appoauth.CredentialTokenRepository
	providerIDs  []string
}

func resolveOAuthRuntime(ctx context.Context, cfg config.Config, snapshot domaincatalog.Snapshot, store *sqlitestore.Store, logger *slog.Logger) (oauthRuntime, error) {
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
	exchanger := provideroauth.NewExchanger(nil)

	configured := make(map[string]struct{}, len(snapshot.Providers))
	for _, item := range snapshot.Providers {
		configured[item.ID] = struct{}{}
	}
	profiles := make(map[string]domainoauth.Provider)
	providerIDs := make([]string, 0, len(oauthConfig.Profiles)+1)
	addProfile := func(profile domainoauth.Provider) error {
		if _, ok := configured[profile.ID]; !ok {
			return fmt.Errorf("OAuth profile %q does not match a configured provider", profile.ID)
		}
		if _, exists := profiles[profile.ID]; exists {
			return fmt.Errorf("duplicate OAuth profile %q", profile.ID)
		}
		profiles[profile.ID] = profile
		providerIDs = append(providerIDs, profile.ID)
		return nil
	}

	if oauthConfig.GeminiClientID != "" {
		profile, err := provideroauth.GeminiProfile(provideroauth.ProfileOptions{
			ProviderID: cfg.Gemini.ID,
			ClientID:   oauthConfig.GeminiClientID,
		})
		if err != nil {
			return oauthRuntime{}, err
		}
		if err := addProfile(profile); err != nil {
			return oauthRuntime{}, err
		}
	}
	for _, configuredProfile := range oauthConfig.Profiles {
		options := provideroauth.ProfileOptions{
			ProviderID:          configuredProfile.ProviderID,
			ClientID:            configuredProfile.ClientID,
			Scopes:              configuredProfile.Scopes,
			AuthorizationURL:    configuredProfile.AuthorizationURL,
			TokenURL:            configuredProfile.TokenURL,
			AuthorizationParams: configuredProfile.AuthorizationParams,
		}
		var profile domainoauth.Provider
		if configuredProfile.Kind == "gemini" {
			profile, err = provideroauth.GeminiProfile(options)
		} else {
			profile, err = provideroauth.GenericProfile(options)
		}
		if err != nil {
			return oauthRuntime{}, err
		}
		if err := addProfile(profile); err != nil {
			return oauthRuntime{}, err
		}
	}

	service := appoauth.NewService(appoauth.NewMemoryStateRepository(), exchanger, tokens)
	handler, err := oauthhttp.NewWithLogger(service, profiles, oauthConfig.PublicBaseURL, logger)
	if err != nil {
		return oauthRuntime{}, err
	}
	return oauthRuntime{
		handler:      handler,
		bearerTokens: appoauth.NewRuntimeTokenResolver(tokens, exchanger, profiles),
		tokens:       tokens,
		providerIDs:  providerIDs,
	}, nil
}
