package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	domainprovider "github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/handler/oauth"
	provideroauth "github.com/phongsathornpt/kokekokkor/internal/provider/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/repository/memory"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/repository/sqlite"
	"github.com/phongsathornpt/kokekokkor/internal/security/secretbox"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
)

type tokenStoreRepo interface {
	Get(context.Context, string) (domainoauth.TokenSet, error)
	Put(context.Context, string, domainoauth.TokenSet) error
	Delete(context.Context, string) error
}

type oauthRuntime struct {
	handler      http.Handler
	service      *appoauth.Service
	bearerTokens *appoauth.RuntimeTokenResolver
	tokens       tokenStoreRepo
	profiles     map[string]domainoauth.Provider
	providerIDs  []string
}

func resolveOAuthRuntime(ctx context.Context, cfg config.Config, snapshot domaincatalog.Snapshot, store *sqlitestore.Store, logger *slog.Logger) (oauthRuntime, error) {
	oauthConfig, enabled, err := config.LoadOAuth()
	if err != nil || !enabled {
		return oauthRuntime{}, err
	}

	var tokens tokenStoreRepo
	if store != nil {
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
		tokens = appoauth.NewCredentialTokenRepository(credentials)
	} else {
		tokens = memory.New().Tokens()
	}
	exchanger := provideroauth.NewExchanger(nil)

	configured := make(map[string]struct{}, len(snapshot.Providers))
	for _, item := range snapshot.Providers {
		configured[item.ID] = struct{}{}
	}
	profiles := make(map[string]domainoauth.Provider)
	providerIDs := make([]string, 0, len(oauthConfig.Profiles)+4)
	addProfile := func(profile domainoauth.Provider) error {
		if store != nil {
			_, configuredProvider := configured[profile.ID]
			if !configuredProvider && !builtinOAuthProviderID(profile.ID) {
				return fmt.Errorf("OAuth profile %q does not match a configured provider", profile.ID)
			}
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
	if oauthConfig.CodexClientID != "" {
		providerID := defaultOpenAIProviderID(cfg, snapshot)
		profile, err := provideroauth.CodexProfile(provideroauth.ProfileOptions{
			ProviderID: providerID,
			ClientID:   oauthConfig.CodexClientID,
		})
		if err != nil {
			return oauthRuntime{}, err
		}
		if err := addProfile(profile); err != nil {
			return oauthRuntime{}, err
		}
	}
	if oauthConfig.ClaudeClientID != "" {
		providerID := defaultAnthropicProviderID(cfg, snapshot)
		profile, err := provideroauth.ClaudeProfile(provideroauth.ProfileOptions{
			ProviderID: providerID,
			ClientID:   oauthConfig.ClaudeClientID,
		})
		if err != nil {
			return oauthRuntime{}, err
		}
		if err := addProfile(profile); err != nil {
			return oauthRuntime{}, err
		}
	}
	if oauthConfig.GitHubClientID != "" {
		providerID := defaultGitHubProviderID(cfg, snapshot)
		profile, err := provideroauth.GitHubCopilotProfile(provideroauth.ProfileOptions{
			ProviderID: providerID,
			ClientID:   oauthConfig.GitHubClientID,
		})
		if err != nil {
			return oauthRuntime{}, err
		}
		if err := addProfile(profile); err != nil {
			return oauthRuntime{}, err
		}
	}
	if oauthConfig.AntigravityClientID != "" {
		providerID := defaultAntigravityProviderID(cfg, snapshot)
		profile, err := provideroauth.AntigravityProfile(provideroauth.ProfileOptions{
			ProviderID: providerID,
			ClientID:   oauthConfig.AntigravityClientID,
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
			ProviderID:             configuredProfile.ProviderID,
			ClientID:               configuredProfile.ClientID,
			Scopes:                 configuredProfile.Scopes,
			AuthorizationURL:       configuredProfile.AuthorizationURL,
			TokenURL:               configuredProfile.TokenURL,
			DeviceAuthorizationURL: configuredProfile.DeviceAuthorizationURL,
			AuthorizationParams:    configuredProfile.AuthorizationParams,
		}
		var profile domainoauth.Provider
		switch configuredProfile.Kind {
		case "gemini":
			profile, err = provideroauth.GeminiProfile(options)
		case "codex", "chatgpt":
			profile, err = provideroauth.CodexProfile(options)
		case "claude":
			profile, err = provideroauth.ClaudeProfile(options)
		case "github", "copilot":
			profile, err = provideroauth.GitHubCopilotProfile(options)
		case "antigravity", "agy":
			profile, err = provideroauth.AntigravityProfile(options)
		default:
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
		service:      service,
		bearerTokens: appoauth.NewRuntimeTokenResolver(tokens, exchanger, profiles),
		tokens:       tokens,
		profiles:     profiles,
		providerIDs:  providerIDs,
	}, nil
}

func builtinOAuthProviderID(providerID string) bool {
	switch providerID {
	case "openai", "anthropic", "gemini", "antigravity":
		return true
	default:
		return false
	}
}

func defaultAnthropicProviderID(cfg config.Config, snapshot domaincatalog.Snapshot) string {
	if cfg.Anthropic.ID != "" {
		return cfg.Anthropic.ID
	}
	if id, ok := snapshot.Defaults[domainprovider.ProtocolAnthropic]; ok && id != "" {
		return id
	}
	for _, p := range snapshot.Providers {
		if p.Protocol == domainprovider.ProtocolAnthropic {
			return p.ID
		}
	}
	return "anthropic"
}

func defaultGitHubProviderID(cfg config.Config, snapshot domaincatalog.Snapshot) string {
	for _, p := range snapshot.Providers {
		if strings.Contains(strings.ToLower(p.ID), "github") || strings.Contains(strings.ToLower(p.ID), "copilot") {
			return p.ID
		}
	}
	return "github"
}

func defaultAntigravityProviderID(cfg config.Config, snapshot domaincatalog.Snapshot) string {
	for _, p := range snapshot.Providers {
		if strings.Contains(strings.ToLower(p.ID), "antigravity") || strings.Contains(strings.ToLower(p.ID), "agy") {
			return p.ID
		}
	}
	return "antigravity"
}

func defaultOpenAIProviderID(cfg config.Config, snapshot domaincatalog.Snapshot) string {
	if cfg.DefaultProviderID != "" {
		return cfg.DefaultProviderID
	}
	if id, ok := snapshot.Defaults[domainprovider.ProtocolOpenAI]; ok && id != "" {
		return id
	}
	for _, p := range snapshot.Providers {
		if p.Protocol == domainprovider.ProtocolOpenAI {
			return p.ID
		}
	}
	return "openai"
}
