package adminhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdminResponsesIncludeBrowserSecurityHeaders(t *testing.T) {
	auth := NewSessionAuth("secret", time.Hour, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	now := time.Now().UTC()
	auth.now = func() time.Time { return now }
	auth.sessions["session"] = adminSession{CSRF: "csrf", ExpiresAt: now.Add(time.Hour)}

	tests := []struct {
		name string
		req  *http.Request
	}{
		{name: "login", req: httptest.NewRequest(http.MethodGet, "/admin/login", nil)},
		{name: "unauthenticated redirect", req: httptest.NewRequest(http.MethodGet, "/admin", nil)},
		{name: "authenticated admin", req: func() *http.Request {
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			req.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: "session"})
			return req
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			auth.ServeHTTP(rec, tt.req)

			if got := rec.Header().Get("Content-Security-Policy"); got != adminContentSecurityPolicy {
				t.Fatalf("Content-Security-Policy = %q", got)
			}
			if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
				t.Fatalf("Referrer-Policy = %q", got)
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q", got)
			}
			if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
				t.Fatalf("X-Frame-Options = %q", got)
			}
		})
	}
}
