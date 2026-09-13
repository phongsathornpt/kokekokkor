package oauth

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Provider struct {
	ID               string
	AuthorizationURL string
	TokenURL         string
	ClientID         string
	Scopes           []string
	AuthorizationParams map[string]string
}

func (p Provider) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("OAuth provider ID must not be empty")
	}
	if strings.TrimSpace(p.ClientID) == "" {
		return fmt.Errorf("OAuth provider %q client ID must not be empty", p.ID)
	}
	for name, raw := range map[string]string{"authorization URL": p.AuthorizationURL, "token URL": p.TokenURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("OAuth provider %q %s must be an absolute HTTPS URL", p.ID, name)
		}
	}
	return nil
}

type PendingAuthorization struct {
	ProviderID  string
	State       string
	CodeVerifier string
	RedirectURI string
	CreatedAt   time.Time
	ExpiresAt   time.Time
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
