package config

import (
	"os"
	"time"
)

const defaultAdminSessionTTL = 8 * time.Hour

type Admin struct {
	Password   string
	SessionTTL time.Duration
}

func LoadAdmin(gatewayAPIKey string) Admin {
	password := os.Getenv("KOKEKOKKOR_ADMIN_PASSWORD")
	if password == "" {
		password = gatewayAPIKey
	}
	return Admin{Password: password, SessionTTL: defaultAdminSessionTTL}
}
