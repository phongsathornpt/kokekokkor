package adminhttp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAdminLoginEvictsOldestSessionAtCapacity(t *testing.T) {
	auth := NewSessionAuth("secret", time.Hour, http.NotFoundHandler())
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }
	for i := 0; i < maxAdminSessions; i++ {
		token := fmt.Sprintf("session-%02d", i)
		auth.sessions[token] = adminSession{CSRF: "csrf", ExpiresAt: now.Add(time.Duration(i+1) * time.Minute)}
	}

	form := url.Values{"password": {"secret"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if len(auth.sessions) != maxAdminSessions {
		t.Fatalf("sessions = %d, want %d", len(auth.sessions), maxAdminSessions)
	}
	if _, ok := auth.sessions["session-00"]; ok {
		t.Fatal("oldest session was not evicted")
	}
}

func TestAdminLoginPrunesExpiredSessionsBeforeEviction(t *testing.T) {
	auth := NewSessionAuth("secret", time.Hour, http.NotFoundHandler())
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }
	auth.sessions["expired"] = adminSession{CSRF: "csrf", ExpiresAt: now.Add(-time.Minute)}
	for i := 0; i < maxAdminSessions-1; i++ {
		token := fmt.Sprintf("live-%02d", i)
		auth.sessions[token] = adminSession{CSRF: "csrf", ExpiresAt: now.Add(time.Hour)}
	}

	form := url.Values{"password": {"secret"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if _, ok := auth.sessions["expired"]; ok {
		t.Fatal("expired session was not pruned")
	}
	if _, ok := auth.sessions["live-00"]; !ok {
		t.Fatal("live session was evicted even though expired capacity was available")
	}
	if len(auth.sessions) != maxAdminSessions {
		t.Fatalf("sessions = %d, want %d", len(auth.sessions), maxAdminSessions)
	}
}
