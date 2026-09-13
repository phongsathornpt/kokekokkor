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

type closer interface {
	Close() error
}

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
	targets, defaults, modelRoutes := routingInputs(snapshot, cfg)
	router, err := routing.NewProtocolTable(targets, defaults, modelRoutes)
	if err != nil {
		if catalogStore != nil {
			_ = catalogStore.Close()
		}
		return nil, err
	}

	anthropicTarget := protocolDefaultTarget(targets, defaults, "anthropic")
	geminiTarget := protocolDefaultTarget(targets, defaults, "gemini")

	bufferedUpstream := upstreamhttp.New(cfg.Anthropic.Version)
	crossProtocol := translator.New(bufferedUpstream)

	openAIUpstream := openaicompat.New(logger)
	openAI := openaiProtocol.NewHandler(router, openAIUpstream, crossProtocol)

	anthropicUpstream := anthropicProvider.New(logger, cfg.Anthropic.Version)
	anthropicAPI := anthropicProtocol.NewRoutedHandler(router, anthropicTarget, anthropicUpstream, crossProtocol)

	geminiUpstream := geminiProvider.New(logger)
	geminiAPI := geminiProtocol.NewRoutedHandler(router, geminiTarget, geminiUpstream, crossProtocol)

	oauthHandler, err := resolveOAuthHandler(context.Background(), cfg, catalogStore)
	if err != nil {
		if catalogStore != nil {
			_ = catalogStore.Close()
		}
		return nil, err
	}
	server := httpserver.New(cfg.HTTP.Addr, cfg.GatewayAPIKey, router.Ready, openAI, anthropicAPI, geminiAPI, oauthHandler, logger)
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

func (a *App) Close() error {
	if a == nil || a.catalogStore == nil {
		return nil
	}
	err := a.catalogStore.Close()
	a.catalogStore = nil
	return err
}
