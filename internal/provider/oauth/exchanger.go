package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
)

const (
	defaultTokenRequestTimeout = 15 * time.Second
	maxOAuthErrorBodyBytes     = 16 << 10
	maxOAuthTokenBodyBytes     = 1 << 20
)

type Exchanger struct {
	client *http.Client
	now    func() time.Time
}

func NewExchanger(client *http.Client) *Exchanger {
	if client == nil {
		client = &http.Client{Timeout: defaultTokenRequestTimeout}
	}
	return &Exchanger{client: client, now: time.Now}
}

func (e *Exchanger) Exchange(ctx context.Context, provider domainoauth.Provider, request appoauth.ExchangeRequest) (domainoauth.TokenSet, error) {
	return e.exchangeForm(ctx, provider.TokenURL, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {provider.ClientID},
		"code":          {request.Code},
		"code_verifier": {request.CodeVerifier},
		"redirect_uri":  {request.RedirectURI},
	})
}

func (e *Exchanger) Refresh(ctx context.Context, provider domainoauth.Provider, current domainoauth.TokenSet) (domainoauth.TokenSet, error) {
	if strings.TrimSpace(current.RefreshToken) == "" {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth refresh token must not be empty")
	}
	return e.exchangeForm(ctx, provider.TokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {provider.ClientID},
		"refresh_token": {current.RefreshToken},
	})
}

func (e *Exchanger) exchangeForm(ctx context.Context, tokenURL string, form url.Values) (domainoauth.TokenSet, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("build OAuth token request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")

	response, err := e.client.Do(httpRequest)
	if err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth token request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domainoauth.TokenSet{}, tokenEndpointError(response.StatusCode, response.Body)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthTokenBodyBytes+1))
	if err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("read OAuth token response: %w", err)
	}
	if len(body) > maxOAuthTokenBodyBytes {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth token response exceeds %d byte limit", maxOAuthTokenBodyBytes)
	}

	var payload struct {
		AccessToken  string          `json:"access_token"`
		RefreshToken string          `json:"refresh_token"`
		TokenType    string          `json:"token_type"`
		Scope        string          `json:"scope"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("decode OAuth token response: %w", err)
	}

	tokens := domainoauth.TokenSet{
		AccessToken:  strings.TrimSpace(payload.AccessToken),
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		Scope:        strings.TrimSpace(payload.Scope),
	}
	if seconds, ok := parseExpiresIn(payload.ExpiresIn); ok && seconds > 0 {
		tokens.ExpiresAt = e.now().UTC().Add(time.Duration(seconds) * time.Second)
	}
	return tokens, nil
}

func tokenEndpointError(status int, body io.Reader) error {
	var payload struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(body, maxOAuthErrorBodyBytes)).Decode(&payload)
	if code := safeOAuthErrorCode(payload.Error); code != "" {
		return fmt.Errorf("OAuth token endpoint returned %d (%s)", status, code)
	}
	return fmt.Errorf("OAuth token endpoint returned %d", status)
}

func safeOAuthErrorCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return ""
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return ""
	}
	return value
}

func parseExpiresIn(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var integer int64
	if err := json.Unmarshal(raw, &integer); err == nil {
		return integer, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, false
	}
	integer, err := strconv.ParseInt(text, 10, 64)
	return integer, err == nil
}
