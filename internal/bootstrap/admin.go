package bootstrap

import (
	"context"
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainprovider "github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/handler/admin"
	"github.com/phongsathornpt/kokekokkor/internal/repository/memory"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/repository/sqlite"
	appcatalog "github.com/phongsathornpt/kokekokkor/internal/usecase/catalog"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
	"github.com/phongsathornpt/kokekokkor/web"
)

func resolveAdminHandler(cfg config.Config, snapshot domaincatalog.Snapshot, credentials *appcredential.Service, oauth oauthRuntime, store *sqlitestore.Store, router *routing.Table) (http.Handler, error) {
	adminConfig := config.LoadAdmin(cfg.GatewayAPIKey)
	if !adminConfig.Enabled || adminConfig.Password == "" {
		return nil, nil
	}

	var catalog *appcatalog.Service
	if store != nil {
		catalog = appcatalog.NewService(store, func(next domaincatalog.Snapshot) error {
			nextTargets, defaults, routes := routingInputs(next, credentials.Snapshot())
			return router.ReplaceProtocols(nextTargets, defaults, routes)
		})
	} else {
		mem := memory.New()
		_ = mem.Replace(context.Background(), snapshot)
		catalog = appcatalog.NewService(mem, func(next domaincatalog.Snapshot) error {
			nextTargets, defaults, routes := routingInputs(next, credentials.Snapshot())
			return router.ReplaceProtocols(nextTargets, defaults, routes)
		})
	}
	handler, err := adminhttp.NewManageable(snapshot, catalog, credentials, oauth.tokens, oauth.providerIDs)
	if err != nil {
		return nil, err
	}
	if oauth.service != nil {
		handler.SetOAuthService(oauth.service, oauth.profiles)
	}

	if oauth.handler != nil && catalog != nil {
		type successRegistrar interface {
			SetOnSuccess(func(context.Context, string))
		}
		if registrar, ok := oauth.handler.(successRegistrar); ok && registrar != nil {
			registrar.SetOnSuccess(func(ctx context.Context, providerID string) {
				snap, err := catalog.Load(ctx)
				if err != nil {
					return
				}
				for _, p := range snap.Providers {
					if p.ID == providerID {
						return
					}
				}
				for _, preset := range web.DefaultProviderPresets() {
					if preset.ID == providerID {
						_ = catalog.CreateProvider(ctx, domaincatalog.Provider{
							ID:       preset.ID,
							Protocol: domainprovider.Protocol(preset.Protocol),
							BaseURL:  preset.BaseURL,
							Enabled:  true,
						})
						return
					}
				}
			})
		}
	}

	protected := http.NewServeMux()
	protected.Handle("/admin", handler)
	protected.Handle("/admin/", handler)
	if oauth.handler != nil {
		protected.Handle("GET /admin/oauth/{provider}/start", oauth.handler)
	}
	return adminhttp.NewSessionAuth(adminConfig.Password, adminConfig.SessionTTL, protected), nil
}
