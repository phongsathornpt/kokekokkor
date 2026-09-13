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

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

type Exchanger struct {
	client *http.Client
	now    func() time.Time
}

func NewExchanger(client *http.Client) *Exchanger {
	if client == nil {
		client = http.DefaultClient
	}
	return &Exchanger{client: client, now: time.Now}
}

func (e *Exchanger) Exchange(ctx context.Context, provider domainoauth.Provider, request appoauth.ExchangeRequest) (domainoauth.TokenSet, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {provider.ClientID},
		"code":          {request.Code},
		"code_verifier": {request.CodeVerifier},
		"redirect_uri":  {request.RedirectURI},
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.TokenURL, strings.NewReader(form.Encode()))
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
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth token endpoint returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken  string          `json:"access_token"`
		RefreshToken string          `json:"refresh_token"`
		TokenType    string          `json:"token_type"`
		Scope        string          `json:"scope"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
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
