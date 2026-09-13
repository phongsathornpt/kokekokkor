package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type CredentialEncryption struct {
	ActiveKeyVersion string
	Keys             map[string][]byte
}

func LoadCredentialEncryption() (CredentialEncryption, bool, error) {
	raw := strings.TrimSpace(os.Getenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON"))
	active := strings.TrimSpace(os.Getenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION"))
	if raw == "" && active == "" {
		return CredentialEncryption{}, false, nil
	}
	if raw == "" || active == "" {
		return CredentialEncryption{}, false, fmt.Errorf("KOKEKOKKOR_CREDENTIAL_KEYS_JSON and KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION must be configured together")
	}
	var encoded map[string]string
	if err := json.Unmarshal([]byte(raw), &encoded); err != nil {
		return CredentialEncryption{}, false, fmt.Errorf("parse KOKEKOKKOR_CREDENTIAL_KEYS_JSON: %w", err)
	}
	keys := make(map[string][]byte, len(encoded))
	for version, value := range encoded {
		version = strings.TrimSpace(version)
		if version == "" {
			return CredentialEncryption{}, false, fmt.Errorf("credential key version must not be empty")
		}
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return CredentialEncryption{}, false, fmt.Errorf("decode credential key %q: %w", version, err)
		}
		if len(key) != 32 {
			return CredentialEncryption{}, false, fmt.Errorf("credential key %q must decode to 32 bytes", version)
		}
		keys[version] = key
	}
	if _, ok := keys[active]; !ok {
		return CredentialEncryption{}, false, fmt.Errorf("active credential key version %q is not present in KOKEKOKKOR_CREDENTIAL_KEYS_JSON", active)
	}
	return CredentialEncryption{ActiveKeyVersion: active, Keys: keys}, true, nil
}
