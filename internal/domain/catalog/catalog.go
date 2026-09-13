package catalog

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type Provider struct {
	ID       string
	Protocol provider.Protocol
	BaseURL  string
	Enabled  bool
}

type RouteTarget struct {
	ProviderID string
	Model      string
}

type Snapshot struct {
	Providers []Provider
	Defaults  map[provider.Protocol]string
	Routes    map[string][]RouteTarget
}

func (s Snapshot) Validate() error {
	providers := make(map[string]Provider, len(s.Providers))
	for _, item := range s.Providers {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("provider ID must not be empty")
		}
		switch item.Protocol {
		case provider.ProtocolOpenAI, provider.ProtocolAnthropic, provider.ProtocolGemini:
		default:
			return fmt.Errorf("provider %q uses unsupported protocol %q", item.ID, item.Protocol)
		}
		if err := validateBaseURL(item.BaseURL); err != nil {
			return fmt.Errorf("provider %q: %w", item.ID, err)
		}
		if _, exists := providers[item.ID]; exists {
			return fmt.Errorf("duplicate provider ID %q", item.ID)
		}
		providers[item.ID] = item
	}
	for protocol, providerID := range s.Defaults {
		switch protocol {
		case provider.ProtocolOpenAI, provider.ProtocolAnthropic, provider.ProtocolGemini:
		default:
			return fmt.Errorf("default uses unsupported protocol %q", protocol)
		}
		configured, ok := providers[providerID]
		if !ok {
			return fmt.Errorf("default %q references unknown provider %q", protocol, providerID)
		}
		if configured.Protocol != protocol {
			return fmt.Errorf("default %q references provider %q with protocol %q", protocol, providerID, configured.Protocol)
		}
		if !configured.Enabled {
			return fmt.Errorf("default %q references disabled provider %q", protocol, providerID)
		}
	}
	for model, targets := range s.Routes {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("route model must not be empty")
		}
		if len(targets) == 0 {
			return fmt.Errorf("route %q must contain at least one target", model)
		}
		for _, target := range targets {
			configured, ok := providers[target.ProviderID]
			if !ok {
				return fmt.Errorf("route %q references unknown provider %q", model, target.ProviderID)
			}
			if !configured.Enabled {
				return fmt.Errorf("route %q references disabled provider %q", model, target.ProviderID)
			}
		}
	}
	return nil
}

func validateBaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("base URL must not be empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base URL must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("base URL must include a host")
	}
	return nil
}
