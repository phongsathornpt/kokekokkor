package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
	anthropicProvider "github.com/phongsathornpt/kokekokkor/internal/provider/anthropic"
	"github.com/phongsathornpt/kokekokkor/internal/provider/openaicompat"
	"github.com/phongsathornpt/kokekokkor/internal/translator"
	"github.com/phongsathornpt/kokekokkor/internal/transport/httpserver"
	"github.com/phongsathornpt/kokekokkor/internal/transport/upstreamhttp"
)

type App struct {
	server *http.Server
	logger *slog.Logger
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	targets := make([]provider.Target, 0, len(cfg.Providers)+1)
	for _, configured := range cfg.Providers {
		targets = append(targets, provider.Target{
			ID:       configured.ID,
			Protocol: provider.ProtocolOpenAI,
			BaseURL:  configured.BaseURL,
			APIKey:   configured.APIKey,
		})
	}

	var anthropicTarget *provider.Target
	if cfg.Anthropic.BaseURL != "" {
		target := provider.Target{
			ID:       cfg.Anthropic.ID,
			Protocol: provider.ProtocolAnthropic,
			BaseURL:  cfg.Anthropic.BaseURL,
			APIKey:   cfg.Anthropic.APIKey,
		}
		anthropicTarget = &target
		targets = append(targets, target)
	}

	modelRoutes := make(map[string][]routing.RouteTarget, len(cfg.ModelRoutes))
	for model, configuredTargets := range cfg.ModelRoutes {
		routeTargets := make([]routing.RouteTarget, 0, len(configuredTargets))
		for _, configuredTarget := range configuredTargets {
			routeTargets = append(routeTargets, routing.RouteTarget{
				ProviderID: configuredTarget.ProviderID,
				Model:      configuredTarget.Model,
			})
		}
		modelRoutes[model] = routeTargets
	}

	defaults := make(map[provider.Protocol]string, 2)
	if cfg.DefaultProviderID != "" {
		defaults[provider.ProtocolOpenAI] = cfg.DefaultProviderID
	}
	if anthropicTarget != nil {
		defaults[provider.ProtocolAnthropic] = anthropicTarget.ID
	}
	router, err := routing.NewProtocolTable(targets, defaults, modelRoutes)
	if err != nil {
		return nil, err
	}

	bufferedUpstream := upstreamhttp.New(cfg.Anthropic.Version)
	crossProtocol := translator.New(bufferedUpstream)

	openAIUpstream := openaicompat.New(logger)
	openAI := openaiProtocol.NewHandler(router, openAIUpstream, crossProtocol)

	anthropicUpstream := anthropicProvider.New(logger, cfg.Anthropic.Version)
	anthropicAPI := anthropicProtocol.NewRoutedHandler(router, anthropicTarget, anthropicUpstream, crossProtocol)

	server := httpserver.New(cfg.HTTP.Addr, cfg.GatewayAPIKey, router.Ready, openAI, anthropicAPI, logger)
	return &App{server: server.HTTP, logger: logger}, nil
}

func (a *App) Run(ctx context.Context) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.server.Addr)
	if err != nil {
		return err
	}

	a.logger.Info("gateway listening", "addr", listener.Addr().String())
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}
