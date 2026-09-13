package oauth

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

type fakeExchanger struct {
	request ExchangeRequest
	tokens  domainoauth.TokenSet
	err     error
}

func (f *fakeExchanger) Exchange(_ context.Context, _ domainoauth.Provider, request ExchangeRequest) (domainoauth.TokenSet, error) {
	f.request = request
	return f.tokens, f.err
}

type memoryTokens struct {
	items map[string]domainoauth.TokenSet
}

func (m *memoryTokens) Put(_ context.Context, providerID string, tokens domainoauth.TokenSet) error {
	m.items[providerID] = tokens
	return nil
}

func (m *memoryTokens) Get(_ context.Context, providerID string) (domainoauth.TokenSet, error) {
	tokens, ok := m.items[providerID]
	if !ok {
		return domainoauth.TokenSet{}, errors.New("missing token set")
	}
	return tokens, nil
}

func testProvider() domainoauth.Provider {
	return domainoauth.Provider{
		ID:                  "provider-a",
		AuthorizationURL:    "https://auth.example.com/authorize",
		TokenURL:            "https://auth.example.com/token",
		ClientID:            "client-id",
		Scopes:              []string{"scope:a", "scope:b"},
		AuthorizationParams: map[string]string{"access_type": "offline"},
	}
}

func TestServiceBeginBuildsPKCEAuthorization(t *testing.T) {
	states := NewMemoryStateRepository()
	service := NewService(states, &fakeExchanger{}, &memoryTokens{items: make(map[string]domainoauth.TokenSet)})
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	authorization, err := service.Begin(context.Background(), testProvider(), "http://127.0.0.1:8080/oauth/callback")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	u, err := url.Parse(authorization.URL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	query := u.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "client-id" {
		t.Fatalf("authorization query = %v", query)
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatalf("PKCE query = %v", query)
	}
	if query.Get("state") == "" || query.Get("scope") != "scope:a scope:b" || query.Get("access_type") != "offline" {
		t.Fatalf("authorization query = %v", query)
	}
	pending, err := states.Consume(context.Background(), query.Get("state"))
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if CodeChallengeS256(pending.CodeVerifier) != query.Get("code_challenge") {
		t.Fatal("code challenge does not match stored verifier")
	}
	if !authorization.ExpiresAt.Equal(now.Add(defaultStateTTL)) {
		t.Fatalf("ExpiresAt = %v", authorization.ExpiresAt)
	}
}

func TestServiceCompleteConsumesStateAndPersistsTokens(t *testing.T) {
	states := NewMemoryStateRepository()
	exchanger := &fakeExchanger{tokens: domainoauth.TokenSet{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer"}}
	tokens := &memoryTokens{items: make(map[string]domainoauth.TokenSet)}
	service := NewService(states, exchanger, tokens)

	authorization, err := service.Begin(context.Background(), testProvider(), "https://gateway.example.com/oauth/callback")
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	u, _ := url.Parse(authorization.URL)
	state := u.Query().Get("state")
	result, err := service.Complete(context.Background(), testProvider(), state, "authorization-code", "https://gateway.example.com/oauth/callback")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.AccessToken != "access" || tokens.items["provider-a"].RefreshToken != "refresh" {
		t.Fatalf("tokens = %#v", result)
	}
	if exchanger.request.Code != "authorization-code" || exchanger.request.CodeVerifier == "" {
		t.Fatalf("exchange request = %#v", exchanger.request)
	}
	if _, err := service.Complete(context.Background(), testProvider(), state, "authorization-code", "https://gateway.example.com/oauth/callback"); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("replay error = %v, want ErrStateNotFound", err)
	}
}

func TestMemoryStateRepositoryRejectsExpiredState(t *testing.T) {
	repository := NewMemoryStateRepository()
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	repository.now = func() time.Time { return now }
	if err := repository.Put(context.Background(), domainoauth.PendingAuthorization{State: "state", ExpiresAt: now}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if _, err := repository.Consume(context.Background(), "state"); !errors.Is(err, ErrStateExpired) {
		t.Fatalf("Consume() error = %v, want ErrStateExpired", err)
	}
	if _, err := repository.Consume(context.Background(), "state"); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("second Consume() error = %v, want ErrStateNotFound", err)
	}
}

func TestServiceRejectsReservedAuthorizationParameter(t *testing.T) {
	provider := testProvider()
	provider.AuthorizationParams["state"] = "override"
	service := NewService(NewMemoryStateRepository(), &fakeExchanger{}, &memoryTokens{items: make(map[string]domainoauth.TokenSet)})
	if _, err := service.Begin(context.Background(), provider, "https://gateway.example.com/oauth/callback"); err == nil {
		t.Fatal("Begin() error = nil, want reserved parameter rejection")
	}
}
