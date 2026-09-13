package config

import (
	"os"
	"strings"
	"time"
)

const defaultAdminSessionTTL = 8 * time.Hour

type Admin struct {
	Password   string
	SessionTTL time.Duration
}

func LoadAdmin(gatewayAPIKey string) Admin {
	password := strings.TrimSpace(os.Getenv("KOKEKOKKOR_ADMIN_PASSWORD"))
	if password == "" {
		password = strings.TrimSpace(gatewayAPIKey)
	}
	return Admin{Password: password, SessionTTL: defaultAdminSessionTTL}
}
