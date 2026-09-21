package oauth

import (
	"context"
	"errors"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

var (
	ErrStateNotFound = errors.New("OAuth state not found")
	ErrStateExpired  = errors.New("OAuth state expired")
)

type StateRepository interface {
	Put(context.Context, domainoauth.PendingAuthorization) error
	Consume(context.Context, string) (domainoauth.PendingAuthorization, error)
}

type Exchanger interface {
	Exchange(context.Context, domainoauth.Provider, ExchangeRequest) (domainoauth.TokenSet, error)
}

type TokenRepository interface {
	Put(context.Context, string, domainoauth.TokenSet) error
	Get(context.Context, string) (domainoauth.TokenSet, error)
}

type ExchangeRequest struct {
	Code         string
	CodeVerifier string
	RedirectURI  string
}
