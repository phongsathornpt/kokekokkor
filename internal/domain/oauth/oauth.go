package oauth

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type FlowType string

const (
	FlowTypeAuthorizationCode FlowType = "authorization_code"
	FlowTypeDeviceCode        FlowType = "device_code"
)

type Provider struct {
	ID                     string
	FlowType               FlowType
	AuthorizationURL       string
	TokenURL               string
	DeviceAuthorizationURL string
	ClientID               string
	Scopes                 []string
	AuthorizationParams    map[string]string
}

func (p Provider) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("OAuth provider ID must not be empty")
	}
	if strings.TrimSpace(p.ClientID) == "" {
		return fmt.Errorf("OAuth provider %q client ID must not be empty", p.ID)
	}
	if p.FlowType == FlowTypeDeviceCode {
		for name, raw := range map[string]string{"device authorization URL": p.DeviceAuthorizationURL, "token URL": p.TokenURL} {
			u, err := url.Parse(raw)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				return fmt.Errorf("OAuth provider %q %s must be an absolute HTTPS URL", p.ID, name)
			}
		}
		return nil
	}
	for name, raw := range map[string]string{"authorization URL": p.AuthorizationURL, "token URL": p.TokenURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("OAuth provider %q %s must be an absolute HTTPS URL", p.ID, name)
		}
	}
	return nil
}

type DeviceAuthorization struct {
	DeviceCode              string    `json:"device_code"`
	UserCode                string    `json:"user_code"`
	VerificationURI         string    `json:"verification_uri"`
	VerificationURIComplete string    `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int       `json:"expires_in"`
	Interval                int       `json:"interval"`
	ExpiresAt               time.Time `json:"expires_at"`
}

type PendingAuthorization struct {
	ProviderID   string
	State        string
	CodeVerifier string
	RedirectURI  string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

type Authorization struct {
	URL       string
	ExpiresAt time.Time
}

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	ExpiresAt    time.Time
}
