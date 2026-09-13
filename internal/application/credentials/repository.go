package credentials

import (
	"context"
	"errors"

	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
)

var ErrNotFound = errors.New("credential not found")

type Repository interface {
	Get(context.Context, credential.Ref) ([]byte, error)
	Put(context.Context, credential.Ref, []byte) error
	Delete(context.Context, credential.Ref) error
}
