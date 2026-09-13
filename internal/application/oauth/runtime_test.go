package oauth

import (
	"context"
	"testing"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

type runtimeTokenRepo struct {
	tokens domainoauth.TokenSet
	puts   int
}

func (r *runtimeTokenRepo) Put(_ context.Context, _ string, tokens domainoauth.TokenSet) error {
	r.tokens = tokens
	r.puts++
	return nil
}

func (r *runtimeTokenRepo) Get(context.Context, string) (domainoauth.TokenSet, error) {
	if r.tokens.AccessToken == "" {
		return domainoauth.TokenSet{}, ErrTokenNotFound
	}
	return r.tokens, nil
}

type runtimeRefresher struct {
	calls int
}

func (r *runtimeRefresher) Refresh(context.Context, domainoauth.Provider, domainoauth.TokenSet) (domainoauth.TokenSet, error) {
	r.calls++
	return domainoauth.TokenSet{AccessToken: "fresh", ExpiresAt: time.Date(2026, 9, 14, 2, 0, 0, 0, time.UTC)}, nil
}

func TestRuntimeTokenResolverReturnsCurrentToken(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	repo := &runtimeTokenRepo{tokens: domainoauth.TokenSet{AccessToken: "current", ExpiresAt: now.Add(time.Hour)}}
	refresher := &runtimeRefresher{}
	resolver := NewRuntimeTokenResolver(repo, refresher, nil)
	resolver.now = func() time.Time { return now }

	token, found, err := resolver.BearerToken(context.Background(), "gemini")
	if err != nil || !found || token != "current" {
		t.Fatalf("BearerToken() token=%q found=%v err=%v", token, found, err)
	}
	if refresher.calls != 0 || repo.puts != 0 {
		t.Fatalf("refresh calls=%d puts=%d", refresher.calls, repo.puts)
	}
}

func TestRuntimeTokenResolverRefreshesAndPersists(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	repo := &runtimeTokenRepo{tokens: domainoauth.TokenSet{AccessToken: "old", RefreshToken: "refresh", TokenType: "Bearer", Scope: "scope", ExpiresAt: now.Add(30 * time.Second)}}
	refresher := &runtimeRefresher{}
	resolver := NewRuntimeTokenResolver(repo, refresher, map[string]domainoauth.Provider{
		"gemini": {ID: "gemini", ClientID: "client", AuthorizationURL: "https://example.com/auth", TokenURL: "https://example.com/token"},
	})
	resolver.now = func() time.Time { return now }

	token, found, err := resolver.BearerToken(context.Background(), "gemini")
	if err != nil || !found || token != "fresh" {
		t.Fatalf("BearerToken() token=%q found=%v err=%v", token, found, err)
	}
	if refresher.calls != 1 || repo.puts != 1 {
		t.Fatalf("refresh calls=%d puts=%d", refresher.calls, repo.puts)
	}
	if repo.tokens.RefreshToken != "refresh" || repo.tokens.Scope != "scope" || repo.tokens.TokenType != "Bearer" {
		t.Fatalf("persisted tokens = %#v", repo.tokens)
	}
}

func TestRuntimeTokenResolverFallsBackWhenMissing(t *testing.T) {
	resolver := NewRuntimeTokenResolver(&runtimeTokenRepo{}, &runtimeRefresher{}, nil)
	token, found, err := resolver.BearerToken(context.Background(), "gemini")
	if err != nil || found || token != "" {
		t.Fatalf("BearerToken() token=%q found=%v err=%v", token, found, err)
	}
}
