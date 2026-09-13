package credential

import (
	"fmt"
	"strings"
)

type Kind string

const (
	KindAPIKey            Kind = "api_key"
	KindOAuthAccessToken  Kind = "oauth_access_token"
	KindOAuthRefreshToken Kind = "oauth_refresh_token"
)

type Ref struct {
	ProviderID string
	Kind       Kind
}

func (r Ref) Validate() error {
	if strings.TrimSpace(r.ProviderID) == "" {
		return fmt.Errorf("credential provider ID must not be empty")
	}
	if strings.TrimSpace(string(r.Kind)) == "" {
		return fmt.Errorf("credential kind must not be empty")
	}
	return nil
}
