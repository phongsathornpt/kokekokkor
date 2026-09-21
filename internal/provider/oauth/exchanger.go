package oauth

import (
	"bytes"
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
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {provider.ClientID},
		"code":          {request.Code},
		"code_verifier": {request.CodeVerifier},
		"redirect_uri":  {request.RedirectURI},
	}
	if provider.ClientSecret != "" {
		form.Set("client_secret", provider.ClientSecret)
	}
	return e.exchangeForm(ctx, provider.TokenURL, form)
}

func (e *Exchanger) Refresh(ctx context.Context, provider domainoauth.Provider, current domainoauth.TokenSet) (domainoauth.TokenSet, error) {
	if strings.TrimSpace(current.RefreshToken) == "" {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth refresh token must not be empty")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {provider.ClientID},
		"refresh_token": {current.RefreshToken},
	}
	if provider.ClientSecret != "" {
		form.Set("client_secret", provider.ClientSecret)
	}
	return e.exchangeForm(ctx, provider.TokenURL, form)
}

func (e *Exchanger) DeviceAuthorize(ctx context.Context, provider domainoauth.Provider) (domainoauth.DeviceAuthorization, error) {
	if strings.TrimSpace(provider.DeviceAuthorizationURL) == "" {
		return domainoauth.DeviceAuthorization{}, fmt.Errorf("provider %q has no device authorization URL", provider.ID)
	}

	if strings.Contains(provider.DeviceAuthorizationURL, "auth.openai.com") {
		reqBody, _ := json.Marshal(map[string]string{"client_id": provider.ClientID})
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.DeviceAuthorizationURL, bytes.NewReader(reqBody))
		if err != nil {
			return domainoauth.DeviceAuthorization{}, fmt.Errorf("build device authorization request: %w", err)
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Accept", "application/json")

		response, err := e.client.Do(httpRequest)
		if err != nil {
			return domainoauth.DeviceAuthorization{}, fmt.Errorf("device authorization request: %w", err)
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return domainoauth.DeviceAuthorization{}, tokenEndpointError(response.StatusCode, response.Body)
		}

		var payload struct {
			DeviceAuthID string `json:"device_auth_id"`
			UserCode     string `json:"user_code"`
			Interval     string `json:"interval"`
			ExpiresAt    string `json:"expires_at"`
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			return domainoauth.DeviceAuthorization{}, fmt.Errorf("decode device authorization response: %w", err)
		}
		interval := 5
		if inv, err := strconv.Atoi(payload.Interval); err == nil && inv > 0 {
			interval = inv
		}
		return domainoauth.DeviceAuthorization{
			DeviceCode:              payload.DeviceAuthID + "|" + payload.UserCode,
			UserCode:                payload.UserCode,
			VerificationURI:         "https://auth.openai.com/codex/device",
			VerificationURIComplete: "https://auth.openai.com/codex/device",
			Interval:                interval,
			ExpiresAt:               e.now().UTC().Add(15 * time.Minute),
		}, nil
	}

	form := url.Values{
		"client_id": {provider.ClientID},
	}
	if len(provider.Scopes) > 0 {
		form.Set("scope", strings.Join(provider.Scopes, " "))
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.DeviceAuthorizationURL, strings.NewReader(form.Encode()))
	if err != nil {
		return domainoauth.DeviceAuthorization{}, fmt.Errorf("build device authorization request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")

	response, err := e.client.Do(httpRequest)
	if err != nil {
		return domainoauth.DeviceAuthorization{}, fmt.Errorf("device authorization request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domainoauth.DeviceAuthorization{}, tokenEndpointError(response.StatusCode, response.Body)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthTokenBodyBytes+1))
	if err != nil {
		return domainoauth.DeviceAuthorization{}, fmt.Errorf("read device authorization response: %w", err)
	}

	var payload struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return domainoauth.DeviceAuthorization{}, fmt.Errorf("decode device authorization response: %w", err)
	}
	if payload.DeviceCode == "" || payload.UserCode == "" {
		return domainoauth.DeviceAuthorization{}, fmt.Errorf("device authorization response missing code")
	}
	if payload.Interval <= 0 {
		payload.Interval = 5
	}
	expiresAt := e.now().UTC()
	if payload.ExpiresIn > 0 {
		expiresAt = expiresAt.Add(time.Duration(payload.ExpiresIn) * time.Second)
	} else {
		expiresAt = expiresAt.Add(15 * time.Minute)
	}
	return domainoauth.DeviceAuthorization{
		DeviceCode:              payload.DeviceCode,
		UserCode:                payload.UserCode,
		VerificationURI:         payload.VerificationURI,
		VerificationURIComplete: payload.VerificationURIComplete,
		ExpiresIn:               payload.ExpiresIn,
		Interval:                payload.Interval,
		ExpiresAt:               expiresAt,
	}, nil
}

func (e *Exchanger) DevicePoll(ctx context.Context, provider domainoauth.Provider, deviceCode string) (domainoauth.TokenSet, error) {
	if strings.Contains(provider.DeviceAuthorizationURL, "auth.openai.com") {
		parts := strings.SplitN(deviceCode, "|", 2)
		if len(parts) != 2 {
			return domainoauth.TokenSet{}, fmt.Errorf("invalid OpenAI device code format")
		}
		deviceAuthID, userCode := parts[0], parts[1]
		reqBody, _ := json.Marshal(map[string]string{
			"device_auth_id": deviceAuthID,
			"user_code":      userCode,
		})
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/deviceauth/token", bytes.NewReader(reqBody))
		if err != nil {
			return domainoauth.TokenSet{}, fmt.Errorf("build device token request: %w", err)
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Accept", "application/json")

		response, err := e.client.Do(httpRequest)
		if err != nil {
			return domainoauth.TokenSet{}, fmt.Errorf("device token request: %w", err)
		}
		defer response.Body.Close()

		body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthTokenBodyBytes+1))
		if err != nil {
			return domainoauth.TokenSet{}, fmt.Errorf("read device token response: %w", err)
		}

		if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusNotFound {
			return domainoauth.TokenSet{}, appoauth.ErrAuthorizationPending
		}
		var errorPayload struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &errorPayload)
		if errorPayload.Error.Code == "deviceauth_authorization_pending" {
			return domainoauth.TokenSet{}, appoauth.ErrAuthorizationPending
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return domainoauth.TokenSet{}, tokenEndpointError(response.StatusCode, bytes.NewReader(body))
		}

		var approved struct {
			AuthorizationCode string `json:"authorization_code"`
			CodeVerifier      string `json:"code_verifier"`
		}
		if err := json.Unmarshal(body, &approved); err != nil {
			return domainoauth.TokenSet{}, fmt.Errorf("decode device approval response: %w", err)
		}
		if approved.AuthorizationCode == "" {
			return domainoauth.TokenSet{}, fmt.Errorf("device approval missing authorization code")
		}

		return e.exchangeForm(ctx, provider.TokenURL, url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {provider.ClientID},
			"code":          {approved.AuthorizationCode},
			"code_verifier": {approved.CodeVerifier},
			"redirect_uri":  {"https://auth.openai.com/deviceauth/callback"},
		})
	}

	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {provider.ClientID},
		"device_code": {deviceCode},
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("build device token request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")

	response, err := e.client.Do(httpRequest)
	if err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("device token request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthTokenBodyBytes+1))
	if err != nil {
		return domainoauth.TokenSet{}, fmt.Errorf("read device token response: %w", err)
	}

	var errorPayload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &errorPayload)

	switch errorPayload.Error {
	case "authorization_pending":
		return domainoauth.TokenSet{}, appoauth.ErrAuthorizationPending
	case "slow_down":
		return domainoauth.TokenSet{}, appoauth.ErrSlowDown
	case "access_denied":
		return domainoauth.TokenSet{}, appoauth.ErrAccessDenied
	case "expired_token":
		return domainoauth.TokenSet{}, appoauth.ErrCodeExpired
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domainoauth.TokenSet{}, tokenEndpointError(response.StatusCode, strings.NewReader(string(body)))
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
	if payload.AccessToken == "" {
		return domainoauth.TokenSet{}, fmt.Errorf("OAuth token response did not contain access_token")
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
