package bootstrap

import (
	"context"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/persistence/sqlite"
)

func resolveCatalog(ctx context.Context, cfg config.Config) (domaincatalog.Snapshot, *sqlitestore.Store, error) {
	configured := catalogFromConfig(cfg)
	if cfg.DatabaseDSN == "" {
		return configured, nil, nil
	}
	store, err := sqlitestore.Open(ctx, cfg.DatabaseDSN)
	if err != nil {
		return domaincatalog.Snapshot{}, nil, err
	}
	persisted, err := store.Load(ctx)
	if err != nil {
		_ = store.Close()
		return domaincatalog.Snapshot{}, nil, err
	}
	if catalogIsEmpty(persisted) {
		if err := store.Replace(ctx, configured); err != nil {
			_ = store.Close()
			return domaincatalog.Snapshot{}, nil, err
		}
		return configured, store, nil
	}
	return persisted, store, nil
}

func catalogFromConfig(cfg config.Config) domaincatalog.Snapshot {
	snapshot := domaincatalog.Snapshot{
		Providers: make([]domaincatalog.Provider, 0, len(cfg.Providers)+2),
		Defaults:  make(map[provider.Protocol]string, 3),
		Routes:    make(map[string][]domaincatalog.RouteTarget, len(cfg.ModelRoutes)),
	}
	for _, item := range cfg.Providers {
		snapshot.Providers = append(snapshot.Providers, domaincatalog.Provider{
			ID: item.ID, Protocol: provider.ProtocolOpenAI, BaseURL: item.BaseURL, Enabled: true,
		})
	}
	if cfg.DefaultProviderID != "" {
		snapshot.Defaults[provider.ProtocolOpenAI] = cfg.DefaultProviderID
	}
	if cfg.Anthropic.BaseURL != "" {
		snapshot.Providers = append(snapshot.Providers, domaincatalog.Provider{
			ID: cfg.Anthropic.ID, Protocol: provider.ProtocolAnthropic, BaseURL: cfg.Anthropic.BaseURL, Enabled: true,
		})
		snapshot.Defaults[provider.ProtocolAnthropic] = cfg.Anthropic.ID
	}
	if cfg.Gemini.BaseURL != "" {
		snapshot.Providers = append(snapshot.Providers, domaincatalog.Provider{
			ID: cfg.Gemini.ID, Protocol: provider.ProtocolGemini, BaseURL: cfg.Gemini.BaseURL, Enabled: true,
		})
		snapshot.Defaults[provider.ProtocolGemini] = cfg.Gemini.ID
	}
	for model, configuredTargets := range cfg.ModelRoutes {
		targets := make([]domaincatalog.RouteTarget, 0, len(configuredTargets))
		for _, target := range configuredTargets {
			targets = append(targets, domaincatalog.RouteTarget{ProviderID: target.ProviderID, Model: target.Model})
		}
		snapshot.Routes[model] = targets
	}
	return snapshot
}

func catalogIsEmpty(snapshot domaincatalog.Snapshot) bool {
	return len(snapshot.Providers) == 0 && len(snapshot.Defaults) == 0 && len(snapshot.Routes) == 0
}

func routingInputs(snapshot domaincatalog.Snapshot, cfg config.Config) ([]provider.Target, map[provider.Protocol]string, map[string][]routing.RouteTarget) {
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

	targets := make([]provider.Target, 0, len(snapshot.Providers))
	for _, item := range snapshot.Providers {
		if !item.Enabled {
			continue
		}
		targets = append(targets, provider.Target{
			ID: item.ID, Protocol: item.Protocol, BaseURL: item.BaseURL, APIKey: credentials[item.ID],
		})
	}
	defaults := make(map[provider.Protocol]string, len(snapshot.Defaults))
	for protocolName, providerID := range snapshot.Defaults {
		defaults[protocolName] = providerID
	}
	routes := make(map[string][]routing.RouteTarget, len(snapshot.Routes))
	for model, persistedTargets := range snapshot.Routes {
		routeTargets := make([]routing.RouteTarget, 0, len(persistedTargets))
		for _, target := range persistedTargets {
			routeTargets = append(routeTargets, routing.RouteTarget{ProviderID: target.ProviderID, Model: target.Model})
		}
		routes[model] = routeTargets
	}
	return targets, defaults, routes
}

func protocolDefaultTarget(targets []provider.Target, defaults map[provider.Protocol]string, protocolName provider.Protocol) *provider.Target {
	providerID := defaults[protocolName]
	for _, target := range targets {
		if target.ID == providerID && target.EffectiveProtocol() == protocolName {
			copy := target
			return &copy
		}
	}
	return nil
}
