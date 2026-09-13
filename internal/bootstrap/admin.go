package bootstrap

import (
	"net/http"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/transport/adminhttp"
)

func resolveAdminHandler(snapshot domaincatalog.Snapshot, targets []provider.Target, oauth oauthRuntime) (http.Handler, error) {
	apiKeys := make(map[string]bool, len(targets))
	for _, target := range targets {
		apiKeys[target.ID] = target.APIKey != ""
	}
	return adminhttp.New(snapshot, oauth.tokens, oauth.providerIDs, apiKeys)
}
