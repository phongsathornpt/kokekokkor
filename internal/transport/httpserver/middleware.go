package httpserver

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

func bearerAuth(expected string, next http.Handler) http.Handler {
	if expected == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || !apiKeyEqual(token, expected) {
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

func anthropicAPIKeyAuth(expected string, next http.Handler) http.Handler {
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
		if !apiKeyEqual(token, expected) {
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

func geminiAPIKeyAuth(expected string, next http.Handler) http.Handler {
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
		if !apiKeyEqual(token, expected) {
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

func apiKeyEqual(got, expected string) bool {
	if got == "" || len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 {
			var b [12]byte
			if _, err := rand.Read(b[:]); err == nil {
				id = hex.EncodeToString(b[:])
			}
		}
		if id != "" {
			w.Header().Set("X-Request-ID", id)
		}
		next.ServeHTTP(w, r)
	})
}
