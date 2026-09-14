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
	anthropicProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/anthropic"
	geminiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/gemini"
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
	anthropicProvider "github.com/phongsathornpt/kokekokkor/internal/provider/anthropic"
	geminiProvider "github.com/phongsathornpt/kokekokkor/internal/provider/gemini"
	"github.com/phongsathornpt/kokekokkor/internal/provider/openaicompat"
	"github.com/phongsathornpt/kokekokkor/internal/translator"
	"github.com/phongsathornpt/kokekokkor/internal/transport/httpserver"
	"github.com/phongsathornpt/kokekokkor/internal/transport/upstreamhttp"
)

type closer interface{ Close() error }

type App struct {
	server       *http.Server
	logger       *slog.Logger
	catalogStore closer
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	snapshot, catalogStore, err := resolveCatalog(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	if err := snapshot.Validate(); err != nil {
		if catalogStore != nil {
			_ = catalogStore.Close()
		}
		return nil, err
	}
	credentials, err := resolveCredentials(context.Background(), cfg, snapshot, catalogStore)
	if err != nil {
		if catalogStore != nil {
			_ = catalogStore.Close()
		}
		return nil, err
	}
	targets, defaults, modelRoutes := routingInputs(snapshot, credentials.Snapshot())
	router, err := routing.NewProtocolTable(targets, defaults, modelRoutes)
	if err != nil {
		if catalogStore != nil {
			_ = catalogStore.Close()
		}
		return nil, err
	}
	if catalogStore != nil {
		credentials.SetRuntimeApply(func(ctx context.Context, values map[string]string) error {
			current, err := catalogStore.Load(ctx)
			if err != nil {
				return err
			}
			nextTargets, nextDefaults, nextRoutes := routingInputs(current, values)
			return router.ReplaceProtocols(nextTargets, nextDefaults, nextRoutes)
		})
	}

	oauth, err := resolveOAuthRuntime(context.Background(), cfg, snapshot, catalogStore, logger)
	if err != nil {
		if catalogStore != nil {
			_ = catalogStore.Close()
		}
		return nil, err
	}
	admin, err := resolveAdminHandler(cfg, snapshot, credentials, oauth, catalogStore, router)
	if err != nil {
		if catalogStore != nil {
			_ = catalogStore.Close()
		}
		return nil, err
	}

	anthropicTarget := protocolDefaultTarget(targets, defaults, "anthropic")
	geminiTarget := protocolDefaultTarget(targets, defaults, "gemini")

	bufferedUpstream := upstreamhttp.NewWithBearerTokenResolver(cfg.Anthropic.Version, oauth.bearerTokens)
	crossProtocol := translator.New(bufferedUpstream)

	openAIUpstream := openaicompat.NewWithBearerTokenResolver(logger, oauth.bearerTokens)
	openAI := openaiProtocol.NewHandler(router, openAIUpstream, crossProtocol)

	anthropicUpstream := anthropicProvider.NewWithBearerTokenResolver(logger, cfg.Anthropic.Version, oauth.bearerTokens)
	anthropicAPI := anthropicProtocol.NewRoutedHandler(router, anthropicTarget, anthropicUpstream, crossProtocol)

	geminiUpstream := geminiProvider.NewWithBearerTokenResolver(logger, oauth.bearerTokens)
	geminiAPI := geminiProtocol.NewRoutedHandler(router, geminiTarget, geminiUpstream, crossProtocol)

	server := httpserver.NewWithAdmin(cfg.HTTP.Addr, cfg.GatewayAPIKey, router.Ready, openAI, anthropicAPI, geminiAPI, oauth.handler, admin, logger)
	return &App{server: server.HTTP, logger: logger, catalogStore: catalogStore}, nil
}

func (a *App) Run(ctx context.Context) (err error) {
	defer func() {
		if closeErr := a.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.server.Addr)
	if err != nil {
		return err
	}
	a.logger.Info("gateway listening", "addr", listener.Addr().String())
	errCh := make(chan error, 1)
	go func() { errCh <- a.server.Serve(listener) }()
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

func (a *App) Close() error {
	if a == nil || a.catalogStore == nil {
		return nil
	}
	err := a.catalogStore.Close()
	a.catalogStore = nil
	return err
}
