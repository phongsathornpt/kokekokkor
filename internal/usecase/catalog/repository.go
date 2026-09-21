package catalog

import (
	"context"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
)

type Repository interface {
	Load(context.Context) (domaincatalog.Snapshot, error)
	Replace(context.Context, domaincatalog.Snapshot) error
}
