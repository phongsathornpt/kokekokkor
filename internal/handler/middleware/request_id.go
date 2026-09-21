package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestID ensures every HTTP request has an X-Request-ID header attached to both request and response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 {
			var b [12]byte
			if _, err := rand.Read(b[:]); err == nil {
				id = hex.EncodeToString(b[:])
			}
		}
		if id != "" {
			r.Header.Set("X-Request-ID", id)
			w.Header().Set("X-Request-ID", id)
		}
		next.ServeHTTP(w, r)
	})
}
