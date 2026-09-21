package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// BearerAuth validates that the request Authorization header has a valid Bearer token.
func BearerAuth(expected string, next http.Handler) http.Handler {
	if expected == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := extractBearerToken(r.Header.Get("Authorization"))
		if !ok || !APIKeyEqual(token, expected) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "invalid API key",
					"type":    "authentication_error",
					"code":    "invalid_api_key",
				},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AnthropicAPIKeyAuth validates X-Api-Key or Bearer token for Anthropic requests.
func AnthropicAPIKeyAuth(expected string, next http.Handler) http.Handler {
	if expected == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get("X-Api-Key"))
		if token == "" {
			token, _ = extractBearerToken(r.Header.Get("Authorization"))
		}
		if !APIKeyEqual(token, expected) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type": "error",
				"error": map[string]any{
					"type":    "authentication_error",
					"message": "invalid x-api-key",
				},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GeminiAPIKeyAuth validates X-Goog-Api-Key, query parameter key, or Bearer token.
func GeminiAPIKeyAuth(expected string, next http.Handler) http.Handler {
	if expected == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get("X-Goog-Api-Key"))
		if token == "" {
			token = strings.TrimSpace(r.URL.Query().Get("key"))
		}
		if token == "" {
			token, _ = extractBearerToken(r.Header.Get("Authorization"))
		}
		if !APIKeyEqual(token, expected) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    http.StatusUnauthorized,
					"message": "invalid x-goog-api-key",
					"status":  "UNAUTHENTICATED",
				},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractBearerToken safely extracts a Bearer token from an Authorization header,
// handling variable whitespace between the scheme and token.
func extractBearerToken(authHeader string) (string, bool) {
	trimmed := strings.TrimSpace(authHeader)
	if trimmed == "" {
		return "", false
	}
	scheme, token, ok := strings.Cut(trimmed, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

// APIKeyEqual compares two API keys in constant time without leaking key length.
func APIKeyEqual(got, expected string) bool {
	if got == "" || expected == "" {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) == 1
}
