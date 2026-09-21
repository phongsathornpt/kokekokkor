package oauth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNewExchangerUsesBoundedDefaultClient(t *testing.T) {
	exchanger := NewExchanger(nil)
	if exchanger.client == nil {
		t.Fatal("client = nil")
	}
	if exchanger.client.Timeout != defaultTokenRequestTimeout {
		t.Fatalf("Timeout = %v, want %v", exchanger.client.Timeout, defaultTokenRequestTimeout)
	}
}

func TestExchangerPreservesInjectedClient(t *testing.T) {
	client := &http.Client{Timeout: time.Minute}
	exchanger := NewExchanger(client)
	if exchanger.client != client {
		t.Fatal("NewExchanger replaced injected client")
	}
}

func TestExchangerPostsAuthorizationCodeWithPKCE(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.String() != "https://login.example.com/token" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("client_id") != "client-id" {
			t.Fatalf("form = %v", r.Form)
		}
		if r.Form.Get("code") != "code" || r.Form.Get("code_verifier") != "verifier" || r.Form.Get("redirect_uri") != "https://gateway.example.com/oauth/provider/callback" {
			t.Fatalf("form = %v", r.Form)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","scope":"a b","expires_in":3600}`)),
		}, nil
	})}
	exchanger := NewExchanger(client)
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	exchanger.now = func() time.Time { return now }

	tokens, err := exchanger.Exchange(context.Background(), domainoauth.Provider{
		ID:               "provider",
		AuthorizationURL: "https://login.example.com/authorize",
		TokenURL:         "https://login.example.com/token",
		ClientID:         "client-id",
	}, appoauth.ExchangeRequest{Code: "code", CodeVerifier: "verifier", RedirectURI: "https://gateway.example.com/oauth/provider/callback"})
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh" || tokens.TokenType != "Bearer" || tokens.Scope != "a b" {
		t.Fatalf("tokens = %#v", tokens)
	}
	if !tokens.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %v", tokens.ExpiresAt)
	}
}

func TestExchangerRejectsOversizedTokenResponse(t *testing.T) {
	prefix := `{"access_token":"access"}`
	body := prefix + strings.Repeat(" ", maxOAuthTokenBodyBytes-len(prefix)+1)
	client := tokenResponseClient(body)

	_, err := NewExchanger(client).Exchange(context.Background(), testOAuthProvider(), appoauth.ExchangeRequest{
		Code:         "code",
		CodeVerifier: "verifier",
		RedirectURI:  "https://gateway.example.com/oauth/provider/callback",
	})
	if err == nil || !strings.Contains(err.Error(), "OAuth token response exceeds") {
		t.Fatalf("Exchange() error = %v, want token response size error", err)
	}
}

func TestExchangerRejectsTrailingTokenResponseData(t *testing.T) {
	client := tokenResponseClient(`{"access_token":"access"}{"ignored":true}`)

	_, err := NewExchanger(client).Exchange(context.Background(), testOAuthProvider(), appoauth.ExchangeRequest{
		Code:         "code",
		CodeVerifier: "verifier",
		RedirectURI:  "https://gateway.example.com/oauth/provider/callback",
	})
	if err == nil || !strings.Contains(err.Error(), "decode OAuth token response") {
		t.Fatalf("Exchange() error = %v, want strict JSON decode error", err)
	}
}

func tokenResponseClient(body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
}

func testOAuthProvider() domainoauth.Provider {
	return domainoauth.Provider{
		ID:                     "provider",
		AuthorizationURL:       "https://login.example.com/authorize",
		TokenURL:               "https://login.example.com/token",
		DeviceAuthorizationURL: "https://login.example.com/device/code",
		ClientID:               "client-id",
	}
}

func TestExchangerDeviceAuthorize(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.String() != "https://login.example.com/device/code" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"device_code": "dev-123",
				"user_code": "ABCD-4321",
				"verification_uri": "https://login.example.com/device",
				"expires_in": 600,
				"interval": 5
			}`)),
		}, nil
	})}
	ex := NewExchanger(client)
	auth, err := ex.DeviceAuthorize(context.Background(), testOAuthProvider())
	if err != nil {
		t.Fatalf("DeviceAuthorize() error = %v", err)
	}
	if auth.DeviceCode != "dev-123" || auth.UserCode != "ABCD-4321" || auth.Interval != 5 {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestExchangerDevicePoll(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		client := tokenResponseClient(`{"error": "authorization_pending"}`)
		ex := NewExchanger(client)
		_, err := ex.DevicePoll(context.Background(), testOAuthProvider(), "dev-123")
		if err != appoauth.ErrAuthorizationPending {
			t.Fatalf("DevicePoll() error = %v, want ErrAuthorizationPending", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		client := tokenResponseClient(`{"access_token": "secret-access", "token_type": "bearer"}`)
		ex := NewExchanger(client)
		tokens, err := ex.DevicePoll(context.Background(), testOAuthProvider(), "dev-123")
		if err != nil {
			t.Fatalf("DevicePoll() error = %v", err)
		}
		if tokens.AccessToken != "secret-access" {
			t.Fatalf("tokens = %#v", tokens)
		}
	})

	t.Run("openai_device_flow", func(t *testing.T) {
		openAIProvider := domainoauth.Provider{
			ID:                     "openai",
			ClientID:               OpenAICodexClientID,
			DeviceAuthorizationURL: "https://auth.openai.com/api/accounts/deviceauth/usercode",
			TokenURL:               "https://auth.openai.com/oauth/token",
		}
		// 1. Authorize
		authClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"device_auth_id": "auth-id-1",
					"user_code": "USER-1234",
					"interval": "5"
				}`)),
			}, nil
		})}
		ex := NewExchanger(authClient)
		auth, err := ex.DeviceAuthorize(context.Background(), openAIProvider)
		if err != nil {
			t.Fatalf("DeviceAuthorize() error = %v", err)
		}
		if auth.UserCode != "USER-1234" || auth.DeviceCode != "auth-id-1|USER-1234" {
			t.Fatalf("auth = %#v", auth)
		}

		// 2. Poll & Token Exchange
		pollClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/accounts/deviceauth/token" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"authorization_code": "code-xyz",
						"code_verifier": "verifier-abc"
					}`)),
				}, nil
			}
			if r.URL.Path == "/oauth/token" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"access_token": "oa-access-token",
						"refresh_token": "oa-refresh-token",
						"token_type": "Bearer"
					}`)),
				}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound}, nil
		})}
		exPoll := NewExchanger(pollClient)
		tokens, err := exPoll.DevicePoll(context.Background(), openAIProvider, auth.DeviceCode)
		if err != nil {
			t.Fatalf("DevicePoll() error = %v", err)
		}
		if tokens.AccessToken != "oa-access-token" {
			t.Fatalf("tokens = %#v", tokens)
		}
	})
}
