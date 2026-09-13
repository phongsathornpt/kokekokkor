package bootstrap

import (
	"net/http"

	appcatalog "github.com/phongsathornpt/kokekokkor/internal/application/catalog"
	appcredentials "github.com/phongsathornpt/kokekokkor/internal/application/credentials"
	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/persistence/sqlite"
	"github.com/phongsathornpt/kokekokkor/internal/transport/adminhttp"
)

func resolveAdminHandler(cfg config.Config, snapshot domaincatalog.Snapshot, credentials *appcredentials.Service, oauth oauthRuntime, store *sqlitestore.Store, router *routing.Table) (http.Handler, error) {
	adminConfig := config.LoadAdmin(cfg.GatewayAPIKey)
	if adminConfig.Password == "" {
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
	return adminhttp.NewSessionAuth(adminConfig.Password, adminConfig.SessionTTL, handler), nil
}
