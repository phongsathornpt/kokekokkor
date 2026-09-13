package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	HTTP             HTTP
	GatewayAPIKey    string
	OpenAICompatible OpenAICompatible
}

type HTTP struct {
	Addr string
}

type OpenAICompatible struct {
	ID      string
	BaseURL string
	APIKey  string
}

func Load() Config {
	return Config{
		HTTP: HTTP{Addr: envOr("KOKEKOKKOR_ADDR", ":8080")},
		GatewayAPIKey: os.Getenv("KOKEKOKKOR_API_KEY"),
		OpenAICompatible: OpenAICompatible{
			ID:      envOr("KOKEKOKKOR_OPENAI_PROVIDER_ID", "default"),
			BaseURL: os.Getenv("KOKEKOKKOR_OPENAI_BASE_URL"),
			APIKey:  os.Getenv("KOKEKOKKOR_OPENAI_API_KEY"),
		},
	}
}

func (c Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("http address must not be empty")
	}
	if c.OpenAICompatible.BaseURL == "" {
		return nil
	}

	u, err := url.Parse(c.OpenAICompatible.BaseURL)
	if err != nil {
		return fmt.Errorf("parse OpenAI-compatible base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("OpenAI-compatible base URL must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("OpenAI-compatible base URL must include a host")
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
