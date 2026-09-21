package config

import (
	"os"
	"strings"
	"time"
)

const (
	DefaultAdminPassword   = "admin"
	defaultAdminSessionTTL = 8 * time.Hour
)

type Admin struct {
	Enabled    bool
	Password   string
	SessionTTL time.Duration
}

func LoadAdmin(gatewayAPIKey string) Admin {
	enabled := true
	if raw := strings.TrimSpace(os.Getenv("KOKEKOKKOR_ADMIN_ENABLED")); raw != "" {
		lower := strings.ToLower(raw)
		if lower == "false" || lower == "0" || lower == "off" || lower == "no" {
			enabled = false
		}
	}
	if !enabled {
		return Admin{Enabled: false}
	}

	password := os.Getenv("KOKEKOKKOR_ADMIN_PASSWORD")
	if password == "" {
		password = gatewayAPIKey
	}
	if password == "" {
		password = DefaultAdminPassword
	}
	return Admin{Enabled: true, Password: password, SessionTTL: defaultAdminSessionTTL}
}
