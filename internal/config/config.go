package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTP              HTTP
	GatewayAPIKey     string
	Providers         []OpenAICompatible
	DefaultProviderID string
	ModelRoutes       map[string][]ModelRouteTarget
	Anthropic         Anthropic
}

type HTTP struct {
	Addr string
}

type OpenAICompatible struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type Anthropic struct {
	ID      string
	BaseURL string
	APIKey  string
	Version string
}

type ModelRouteTarget struct {
	ProviderID string `json:"provider"`
	Model      string `json:"model,omitempty"`
}

func Load() (Config, error) {
	cfg := Config{
		HTTP:              HTTP{Addr: envOr("KOKEKOKKOR_ADDR", ":8080")},
		GatewayAPIKey:     os.Getenv("KOKEKOKKOR_API_KEY"),
		DefaultProviderID: os.Getenv("KOKEKOKKOR_DEFAULT_PROVIDER_ID"),
		ModelRoutes:       make(map[string][]ModelRouteTarget),
		Anthropic: Anthropic{
			ID:      envOr("KOKEKOKKOR_ANTHROPIC_PROVIDER_ID", "anthropic"),
			BaseURL: os.Getenv("KOKEKOKKOR_ANTHROPIC_BASE_URL"),
			APIKey:  os.Getenv("KOKEKOKKOR_ANTHROPIC_API_KEY"),
			Version: envOr("KOKEKOKKOR_ANTHROPIC_VERSION", "2023-06-01"),
		},
	}

	if raw := os.Getenv("KOKEKOKKOR_PROVIDERS_JSON"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg.Providers); err != nil {
			return Config{}, fmt.Errorf("parse KOKEKOKKOR_PROVIDERS_JSON: %w", err)
		}
	} else if baseURL := os.Getenv("KOKEKOKKOR_OPENAI_BASE_URL"); baseURL != "" {
		providerID := envOr("KOKEKOKKOR_OPENAI_PROVIDER_ID", "default")
		cfg.Providers = []OpenAICompatible{{
			ID:      providerID,
			BaseURL: baseURL,
			APIKey:  os.Getenv("KOKEKOKKOR_OPENAI_API_KEY"),
		}}
		if cfg.DefaultProviderID == "" {
			cfg.DefaultProviderID = providerID
		}
	}

	if raw := os.Getenv("KOKEKOKKOR_MODEL_ROUTES_JSON"); raw != "" {
		routes, err := parseModelRoutes(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse KOKEKOKKOR_MODEL_ROUTES_JSON: %w", err)
		}
		cfg.ModelRoutes = routes
	}

	if cfg.DefaultProviderID == "" && len(cfg.Providers) == 1 {
		cfg.DefaultProviderID = cfg.Providers[0].ID
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("http address must not be empty")
	}

	providers := make(map[string]struct{}, len(c.Providers))
	for _, configured := range c.Providers {
		if strings.TrimSpace(configured.ID) == "" {
			return fmt.Errorf("provider ID must not be empty")
		}
		if _, exists := providers[configured.ID]; exists {
			return fmt.Errorf("duplicate provider ID %q", configured.ID)
		}
		if err := validateBaseURL(configured.BaseURL); err != nil {
			return fmt.Errorf("provider %q: %w", configured.ID, err)
		}
		providers[configured.ID] = struct{}{}
	}

	if c.DefaultProviderID != "" {
		if _, ok := providers[c.DefaultProviderID]; !ok {
			return fmt.Errorf("default provider %q is not configured", c.DefaultProviderID)
		}
	}

	for model, route := range c.ModelRoutes {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model route name must not be empty")
		}
		if len(route) == 0 {
			return fmt.Errorf("model route %q must have at least one target", model)
		}
		for _, target := range route {
			if _, ok := providers[target.ProviderID]; !ok {
				return fmt.Errorf("model route %q references unknown provider %q", model, target.ProviderID)
			}
		}
	}

	if c.Anthropic.BaseURL != "" {
		if strings.TrimSpace(c.Anthropic.ID) == "" {
			return fmt.Errorf("Anthropic provider ID must not be empty")
		}
		if strings.TrimSpace(c.Anthropic.Version) == "" {
			return fmt.Errorf("Anthropic version must not be empty")
		}
		if err := validateBaseURL(c.Anthropic.BaseURL); err != nil {
			return fmt.Errorf("Anthropic provider %q: %w", c.Anthropic.ID, err)
		}
	}
	return nil
}

func parseModelRoutes(raw string) (map[string][]ModelRouteTarget, error) {
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &encoded); err != nil {
		return nil, err
	}

	routes := make(map[string][]ModelRouteTarget, len(encoded))
	for model, value := range encoded {
		trimmed := strings.TrimSpace(string(value))
		if trimmed == "" {
			return nil, fmt.Errorf("model route %q has an empty value", model)
		}

		switch trimmed[0] {
		case '"':
			var providerID string
			if err := json.Unmarshal(value, &providerID); err != nil {
				return nil, fmt.Errorf("model route %q: %w", model, err)
			}
			routes[model] = []ModelRouteTarget{{ProviderID: providerID}}
		case '{':
			var target ModelRouteTarget
			if err := json.Unmarshal(value, &target); err != nil {
				return nil, fmt.Errorf("model route %q: %w", model, err)
			}
			routes[model] = []ModelRouteTarget{target}
		case '[':
			var targets []ModelRouteTarget
			if err := json.Unmarshal(value, &targets); err != nil {
				return nil, fmt.Errorf("model route %q: %w", model, err)
			}
			routes[model] = targets
		default:
			return nil, fmt.Errorf("model route %q must be a provider string, target object, or target array", model)
		}
	}
	return routes, nil
}

func validateBaseURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("base URL must not be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base URL must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("base URL must include a host")
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
