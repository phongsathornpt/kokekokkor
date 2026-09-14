package adminhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminLoginRejectsOversizedFormBody(t *testing.T) {
	auth := NewSessionAuth("secret", time.Hour, http.NotFoundHandler())
	body := "password=" + strings.Repeat("a", maxAdminFormBodyBytes)
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestAdminMutationRejectsOversizedFormBodyBeforeHandler(t *testing.T) {
	called := false
	auth := NewSessionAuth("secret", time.Hour, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	now := time.Now().UTC()
	auth.now = func() time.Time { return now }
	auth.sessions["session"] = adminSession{CSRF: "csrf", ExpiresAt: now.Add(time.Hour)}

	body := "csrf_token=csrf&value=" + strings.Repeat("a", maxAdminFormBodyBytes)
	req := httptest.NewRequest(http.MethodPost, "/admin/providers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: "session"})
	rec := httptest.NewRecorder()

	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if called {
		t.Fatal("admin mutation handler was called for oversized body")
	}
}
