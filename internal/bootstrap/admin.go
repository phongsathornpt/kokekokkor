package bootstrap

import (
	"net/http"

	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/handler/admin"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/repository/sqlite"
	appcatalog "github.com/phongsathornpt/kokekokkor/internal/usecase/catalog"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/routing"
)

func resolveAdminHandler(cfg config.Config, snapshot domaincatalog.Snapshot, credentials *appcredential.Service, oauth oauthRuntime, store *sqlitestore.Store, router *routing.Table) (http.Handler, error) {
	adminConfig := config.LoadAdmin(cfg.GatewayAPIKey)
	if !adminConfig.Enabled || adminConfig.Password == "" {
		return nil, nil
	}

	var handler *adminhttp.Handler
	var err error
	if store != nil {
		catalog := appcatalog.NewService(store, func(next domaincatalog.Snapshot) error {
			nextTargets, defaults, routes := routingInputs(next, credentials.Snapshot())
			return router.ReplaceProtocols(nextTargets, defaults, routes)
		})
		handler, err = adminhttp.NewManageable(snapshot, catalog, credentials, oauth.tokens, oauth.providerIDs)
	} else {
		handler, err = adminhttp.NewManageable(snapshot, nil, credentials, oauth.tokens, oauth.providerIDs)
	}
	if err != nil {
		return nil, err
	}

	protected := http.NewServeMux()
	protected.Handle("/admin", handler)
	protected.Handle("/admin/", handler)
	if oauth.handler != nil {
		protected.Handle("GET /admin/oauth/{provider}/start", oauth.handler)
	}
	return adminhttp.NewSessionAuth(adminConfig.Password, adminConfig.SessionTTL, protected), nil
}
