package middleware

import (
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
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || !APIKeyEqual(token, expected) {
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
		token := r.Header.Get("X-Api-Key")
		if token == "" {
			scheme, bearer, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			if ok && strings.EqualFold(scheme, "Bearer") {
				token = bearer
			}
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
		token := r.Header.Get("X-Goog-Api-Key")
		if token == "" {
			token = r.URL.Query().Get("key")
		}
		if token == "" {
			scheme, bearer, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			if ok && strings.EqualFold(scheme, "Bearer") {
				token = bearer
			}
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

// APIKeyEqual compares two API keys in constant time.
func APIKeyEqual(got, expected string) bool {
	if got == "" || len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
