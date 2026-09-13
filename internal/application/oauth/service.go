package oauth

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

const defaultStateTTL = 10 * time.Minute

type Service struct {
	states    StateRepository
	exchanger Exchanger
	tokens    TokenRepository
	stateTTL  time.Duration
	now       func() time.Time
}

func NewService(states StateRepository, exchanger Exchanger, tokens TokenRepository) *Service {
	return &Service{
		states:    states,
		exchanger: exchanger,
		tokens:    tokens,
		stateTTL:  defaultStateTTL,
		now:       time.Now,
	}
}

func (s *Service) Begin(ctx context.Context, provider domainoauth.Provider, redirectURI string) (domainoauth.Authorization, error) {
	if err := provider.Validate(); err != nil {
		return domainoauth.Authorization{}, err
	}
	if err := validateRedirectURI(redirectURI); err != nil {
		return domainoauth.Authorization{}, err
	}
	for key := range provider.AuthorizationParams {
		if isReservedAuthorizationParam(key) {
			return domainoauth.Authorization{}, fmt.Errorf("OAuth authorization parameter %q is reserved", key)
		}
	}
	if s.states == nil {
		return domainoauth.Authorization{}, fmt.Errorf("OAuth state repository is not configured")
	}
	state, err := GenerateState()
	if err != nil {
		return domainoauth.Authorization{}, err
	}
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		return domainoauth.Authorization{}, err
	}
	now := s.now().UTC()
	pending := domainoauth.PendingAuthorization{
		ProviderID:   provider.ID,
		State:        state,
		CodeVerifier: verifier,
		RedirectURI:  redirectURI,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.stateTTL),
	}
	if err := s.states.Put(ctx, pending); err != nil {
		return domainoauth.Authorization{}, fmt.Errorf("store OAuth state: %w", err)
	}

	authorizationURL, err := url.Parse(provider.AuthorizationURL)
	if err != nil {
		return domainoauth.Authorization{}, fmt.Errorf("parse OAuth authorization URL: %w", err)
	}
	query := authorizationURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", provider.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("code_challenge", CodeChallengeS256(verifier))
	query.Set("code_challenge_method", "S256")
	if len(provider.Scopes) != 0 {
		query.Set("scope", strings.Join(provider.Scopes, " "))
	}
	for key, value := range provider.AuthorizationParams {
		query.Set(key, value)
	}
	authorizationURL.RawQuery = query.Encode()
	return domainoauth.Authorization{URL: authorizationURL.String(), ExpiresAt: pending.ExpiresAt}, nil
}

func (s *Service) Complete(ctx context.Context, provider domainoauth.Provider, state, code, redirectURI string) (domainoauth.TokenSet, error) {
	if err := provider.Validate(); err != nil {
		return domainoauth.TokenSet{}, err
	}
	if strings.TrimSpace(state) == "" {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth state must not be empty")
	}
	if strings.TrimSpace(code) == "" {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth authorization code must not be empty")
	}
	if s.states == nil || s.exchanger == nil || s.tokens == nil {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth service dependencies are not configured")
	}
	pending, err := s.states.Consume(ctx, state)
	if err != nil {
		return domainoauth.TokenSet{}, err
	}
	if pending.ProviderID != provider.ID {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth state belongs to provider %q, not %q", pending.ProviderID, provider.ID)
	}
	if pending.RedirectURI != redirectURI {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth redirect URI does not match the authorization request")
	}
	tokens, err := s.exchanger.Exchange(ctx, provider, ExchangeRequest{
		Code:         code,
		CodeVerifier: pending.CodeVerifier,
		RedirectURI:  pending.RedirectURI,
	})
	if err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("exchange OAuth authorization code: %w", err)
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth token response did not contain an access token")
	}
	if err := s.tokens.Put(ctx, provider.ID, tokens); err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("persist OAuth token set: %w", err)
	}
	return tokens, nil
}

func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("OAuth redirect URI must be an absolute URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return nil
	}
	return fmt.Errorf("OAuth redirect URI must use HTTPS except for loopback callbacks")
}

func isReservedAuthorizationParam(key string) bool {
	switch strings.ToLower(key) {
	case "response_type", "client_id", "redirect_uri", "state", "scope", "code_challenge", "code_challenge_method":
		return true
	default:
		return false
	}
}
