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
	ModelRoutes       map[string]string
}

type HTTP struct {
	Addr string
}

type OpenAICompatible struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

func Load() (Config, error) {
	cfg := Config{
		HTTP:              HTTP{Addr: envOr("KOKEKOKKOR_ADDR", ":8080")},
		GatewayAPIKey:     os.Getenv("KOKEKOKKOR_API_KEY"),
		DefaultProviderID: os.Getenv("KOKEKOKKOR_DEFAULT_PROVIDER_ID"),
		ModelRoutes:       make(map[string]string),
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
		if err := json.Unmarshal([]byte(raw), &cfg.ModelRoutes); err != nil {
			return Config{}, fmt.Errorf("parse KOKEKOKKOR_MODEL_ROUTES_JSON: %w", err)
		}
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

	for model, providerID := range c.ModelRoutes {
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model route name must not be empty")
		}
		if _, ok := providers[providerID]; !ok {
			return fmt.Errorf("model route %q references unknown provider %q", model, providerID)
		}
	}
	return nil
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
