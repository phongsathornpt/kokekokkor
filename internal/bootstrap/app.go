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
	openaiProtocol "github.com/phongsathornpt/kokekokkor/internal/protocol/openai"
	"github.com/phongsathornpt/kokekokkor/internal/provider/openaicompat"
	"github.com/phongsathornpt/kokekokkor/internal/transport/httpserver"
)

type App struct {
	server *http.Server
	logger *slog.Logger
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var target *provider.Target
	if cfg.OpenAICompatible.BaseURL != "" {
		target = &provider.Target{
			ID:      cfg.OpenAICompatible.ID,
			BaseURL: cfg.OpenAICompatible.BaseURL,
			APIKey:  cfg.OpenAICompatible.APIKey,
		}
	}

	router := routing.NewStatic(target)
	upstream := openaicompat.New(logger)
	openAI := openaiProtocol.NewHandler(router, upstream)
	server := httpserver.New(cfg.HTTP.Addr, cfg.GatewayAPIKey, router.Ready, openAI, logger)

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
