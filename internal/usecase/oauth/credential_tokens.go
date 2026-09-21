package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
)

type CredentialTokenRepository struct {
	credentials appcredential.Repository
}

func NewCredentialTokenRepository(credentials appcredential.Repository) *CredentialTokenRepository {
	return &CredentialTokenRepository{credentials: credentials}
}

func (r *CredentialTokenRepository) Put(ctx context.Context, providerID string, tokens domainoauth.TokenSet) error {
	encoded, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("encode OAuth token set: %w", err)
	}
	return r.credentials.Put(ctx, credential.Ref{ProviderID: providerID, Kind: credential.KindOAuthTokenSet}, encoded)
}

func (r *CredentialTokenRepository) Get(ctx context.Context, providerID string) (domainoauth.TokenSet, error) {
	encoded, err := r.credentials.Get(ctx, credential.Ref{ProviderID: providerID, Kind: credential.KindOAuthTokenSet})
	if errors.Is(err, appcredential.ErrNotFound) {
		return domainoauth.TokenSet{}, ErrTokenNotFound
	}
	if err != nil {
		return domainoauth.TokenSet{}, err
	}
	var tokens domainoauth.TokenSet
	if err := json.Unmarshal(encoded, &tokens); err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("decode OAuth token set: %w", err)
	}
	return tokens, nil
}

func (r *CredentialTokenRepository) Delete(ctx context.Context, providerID string) error {
	return r.credentials.Delete(ctx, credential.Ref{ProviderID: providerID, Kind: credential.KindOAuthTokenSet})
}
