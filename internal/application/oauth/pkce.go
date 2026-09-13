package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const randomTokenBytes = 32

func GenerateState() (string, error) {
	return randomURLToken(randomTokenBytes)
}

func GenerateCodeVerifier() (string, error) {
	return randomURLToken(randomTokenBytes)
}

func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate OAuth random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
