package credential

import (
	"context"
	"errors"

	domaincredential "github.com/phongsathornpt/kokekokkor/internal/domain/credential"
)

var ErrNotFound = errors.New("credential not found")

type Repository interface {
	Get(context.Context, domaincredential.Ref) ([]byte, error)
	Put(context.Context, domaincredential.Ref, []byte) error
	Delete(context.Context, domaincredential.Ref) error
}
