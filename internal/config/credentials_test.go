package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadCredentialEncryptionDisabled(t *testing.T) {
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", "")
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "")
	_, enabled, err := LoadCredentialEncryption()
	if err != nil {
		t.Fatalf("LoadCredentialEncryption() error = %v", err)
	}
	if enabled {
		t.Fatal("enabled = true, want false")
	}
}

func TestLoadCredentialEncryption(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", `{"v1":"`+encoded+`"}`)
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	cfg, enabled, err := LoadCredentialEncryption()
	if err != nil {
		t.Fatalf("LoadCredentialEncryption() error = %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
	}
	if cfg.ActiveKeyVersion != "v1" || len(cfg.Keys["v1"]) != 32 {
		t.Fatalf("config = %#v", cfg)
	}
}

func TestLoadCredentialEncryptionRejectsInvalidKeyLength(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("short"))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", `{"v1":"`+encoded+`"}`)
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	if _, _, err := LoadCredentialEncryption(); err == nil {
		t.Fatal("LoadCredentialEncryption() error = nil, want invalid key length")
	}
}
