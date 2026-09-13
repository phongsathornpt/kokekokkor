package bootstrap

import (
	"net/http"

	appcatalog "github.com/phongsathornpt/kokekokkor/internal/application/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/application/routing"
	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/persistence/sqlite"
	"github.com/phongsathornpt/kokekokkor/internal/transport/adminhttp"
)

func resolveAdminHandler(cfg config.Config, snapshot domaincatalog.Snapshot, targets []provider.Target, oauth oauthRuntime, store *sqlitestore.Store, router *routing.Table) (http.Handler, error) {
	adminConfig := config.LoadAdmin(cfg.GatewayAPIKey)
	if adminConfig.Password == "" {
		return nil, nil
	}
	apiKeys := make(map[string]bool, len(targets))
	for _, target := range targets {
		apiKeys[target.ID] = target.APIKey != ""
	}

	var handler *adminhttp.Handler
	var err error
	if store != nil {
		catalog := appcatalog.NewService(store, func(next domaincatalog.Snapshot) error {
			nextTargets, defaults, routes := routingInputs(next, cfg)
			return router.ReplaceProtocols(nextTargets, defaults, routes)
		})
		handler, err = adminhttp.NewEditable(snapshot, catalog, oauth.tokens, oauth.providerIDs, apiKeys)
	} else {
		handler, err = adminhttp.New(snapshot, oauth.tokens, oauth.providerIDs, apiKeys)
	}
	if err != nil {
		return nil, err
	}
	return adminhttp.NewSessionAuth(adminConfig.Password, adminConfig.SessionTTL, handler), nil
}
