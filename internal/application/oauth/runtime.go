package oauth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

const defaultRefreshSkew = time.Minute

var ErrTokenNotFound = errors.New("OAuth token not found")

type Refresher interface {
	Refresh(context.Context, domainoauth.Provider, domainoauth.TokenSet) (domainoauth.TokenSet, error)
}

type RuntimeTokenResolver struct {
	tokens    TokenRepository
	refresher Refresher
	providers map[string]domainoauth.Provider
	now       func() time.Time
	skew      time.Duration
	mu        sync.Mutex
}

func NewRuntimeTokenResolver(tokens TokenRepository, refresher Refresher, providers map[string]domainoauth.Provider) *RuntimeTokenResolver {
	copied := make(map[string]domainoauth.Provider, len(providers))
	for id, provider := range providers {
		copied[id] = provider
	}
	return &RuntimeTokenResolver{
		tokens: tokens, refresher: refresher, providers: copied,
		now: time.Now, skew: defaultRefreshSkew,
	}
}

func (r *RuntimeTokenResolver) BearerToken(ctx context.Context, providerID string) (string, bool, error) {
	if r == nil || r.tokens == nil {
		return "", false, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tokens, err := r.tokens.Get(ctx, providerID)
	if errors.Is(err, ErrTokenNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load OAuth token set for %q: %w", providerID, err)
	}
	if tokens.AccessToken == "" {
		return "", false, fmt.Errorf("OAuth token set for %q has no access token", providerID)
	}
	if tokens.ExpiresAt.IsZero() || r.now().UTC().Add(r.skew).Before(tokens.ExpiresAt) {
		return tokens.AccessToken, true, nil
	}

	provider, ok := r.providers[providerID]
	if !ok {
		return "", false, fmt.Errorf("OAuth provider profile %q is not configured", providerID)
	}
	if tokens.RefreshToken == "" {
		return "", false, fmt.Errorf("OAuth token for %q expired without a refresh token", providerID)
	}
	if r.refresher == nil {
		return "", false, fmt.Errorf("OAuth refresher is not configured")
	}
	refreshed, err := r.refresher.Refresh(ctx, provider, tokens)
	if err != nil {
		return "", false, fmt.Errorf("refresh OAuth token for %q: %w", providerID, err)
	}
	if refreshed.AccessToken == "" {
		return "", false, fmt.Errorf("OAuth refresh for %q returned no access token", providerID)
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = tokens.RefreshToken
	}
	if refreshed.Scope == "" {
		refreshed.Scope = tokens.Scope
	}
	if refreshed.TokenType == "" {
		refreshed.TokenType = tokens.TokenType
	}
	if err := r.tokens.Put(ctx, providerID, refreshed); err != nil {
		return "", false, fmt.Errorf("persist refreshed OAuth token for %q: %w", providerID, err)
	}
	return refreshed.AccessToken, true, nil
}
