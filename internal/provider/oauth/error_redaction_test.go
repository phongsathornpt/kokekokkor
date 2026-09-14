package oauth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

func TestExchangerRedactsTokenEndpointErrorBody(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"error":"invalid_grant",
				"error_description":"refresh_token=top-secret",
				"access_token":"must-not-leak"
			}`)),
		}, nil
	})}
	exchanger := NewExchanger(client)

	_, err := exchanger.Exchange(context.Background(), domainoauth.Provider{
		TokenURL: "https://login.example.com/token",
		ClientID: "client-id",
	}, appoauth.ExchangeRequest{Code: "bad", CodeVerifier: "verifier", RedirectURI: "https://gateway.example.com/callback"})
	if err == nil {
		t.Fatal("Exchange() error = nil")
	}
	message := err.Error()
	for _, want := range []string{"400", "invalid_grant"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q missing %q", message, want)
		}
	}
	for _, secret := range []string{"top-secret", "refresh_token", "must-not-leak", "access_token"} {
		if strings.Contains(message, secret) {
			t.Fatalf("error leaked %q: %s", secret, message)
		}
	}
}

func TestExchangerOmitsUnsafeOAuthErrorCode(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant secret=oops"}`)),
		}, nil
	})}
	exchanger := NewExchanger(client)

	_, err := exchanger.Exchange(context.Background(), domainoauth.Provider{
		TokenURL: "https://login.example.com/token",
		ClientID: "client-id",
	}, appoauth.ExchangeRequest{Code: "bad", CodeVerifier: "verifier", RedirectURI: "https://gateway.example.com/callback"})
	if err == nil {
		t.Fatal("Exchange() error = nil")
	}
	message := err.Error()
	if message != "OAuth token endpoint returned 401" {
		t.Fatalf("error = %q", message)
	}
	if strings.Contains(message, "oops") {
		t.Fatalf("unsafe provider text leaked: %s", message)
	}
}
