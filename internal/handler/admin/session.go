package adminhttp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/phongsathornpt/kokekokkor/web"
)

const adminSessionCookie = "kokekokkor_admin_session"
const maxAdminFormBodyBytes = 1 << 20
const maxAdminSessions = 64
const adminContentSecurityPolicy = "default-src 'none'; script-src https://cdn.jsdelivr.net; style-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; form-action 'self'; base-uri 'none'; frame-ancestors 'none'; object-src 'none'"

type sessionContextKey struct{}

type adminSession struct {
	CSRF      string
	ExpiresAt time.Time
}

type SessionAuth struct {
	password   string
	ttl        time.Duration
	next       http.Handler
	now        func() time.Time
	mu         sync.Mutex
	mutationMu sync.Mutex
	sessions   map[string]adminSession
}

func NewSessionAuth(password string, ttl time.Duration, next http.Handler) *SessionAuth {
	return &SessionAuth{
		password: password,
		ttl:      ttl,
		next:     next,
		now:      time.Now,
		sessions: make(map[string]adminSession),
	}
}

func (a *SessionAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setAdminSecurityHeaders(w.Header())

	if r.URL.Path == "/admin/login" {
		switch r.Method {
		case http.MethodGet:
			if _, _, ok := a.session(r); ok {
				http.Redirect(w, r, "/admin", http.StatusSeeOther)
				return
			}
			a.renderLogin(w, http.StatusOK, "")
		case http.MethodPost:
			a.login(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	token, session, ok := a.session(r)
	if !ok {
		if cookie, err := r.Cookie(adminSessionCookie); err == nil && cookie.Value != "" {
			a.clearCookie(w, r)
		}
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/admin/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		} else {
			http.Error(w, "admin authentication required", http.StatusUnauthorized)
		}
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if !parseAdminForm(w, r) {
			return
		}
	}

	if r.URL.Path == "/admin/logout" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !csrfEqual(r, session.CSRF) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		a.clearCookie(w, r)
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead && !csrfEqual(r, session.CSRF) {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/admin/login")
		}
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	ctx := context.WithValue(r.Context(), sessionContextKey{}, session.CSRF)
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		a.mutationMu.Lock()
		defer a.mutationMu.Unlock()
	}
	a.next.ServeHTTP(w, r.WithContext(ctx))
}

func AdminCSRFToken(ctx context.Context) string {
	value, _ := ctx.Value(sessionContextKey{}).(string)
	return value
}

func (a *SessionAuth) login(w http.ResponseWriter, r *http.Request) {
	if !parseAdminForm(w, r) {
		return
	}
	password := r.Form.Get("password")
	if !secretEqual(password, a.password) {
		a.renderLogin(w, http.StatusUnauthorized, "invalid password")
		return
	}
	token, err := randomToken(32)
	if err != nil {
		http.Error(w, "session unavailable", http.StatusInternalServerError)
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		http.Error(w, "session unavailable", http.StatusInternalServerError)
		return
	}
	now := a.now().UTC()
	expires := now.Add(a.ttl)
	a.mu.Lock()
	a.pruneLocked(now)
	if len(a.sessions) >= maxAdminSessions {
		a.evictOldestLocked()
	}
	a.sessions[token] = adminSession{CSRF: csrf, ExpiresAt: expires}
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(a.ttl.Seconds()),
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *SessionAuth) session(r *http.Request) (string, adminSession, bool) {
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil || cookie.Value == "" {
		return "", adminSession{}, false
	}
	now := a.now().UTC()
	a.mu.Lock()
	defer a.mu.Unlock()
	session, ok := a.sessions[cookie.Value]
	if !ok || !session.ExpiresAt.After(now) {
		delete(a.sessions, cookie.Value)
		return "", adminSession{}, false
	}
	return cookie.Value, session, true
}

func (a *SessionAuth) pruneLocked(now time.Time) {
	for token, session := range a.sessions {
		if !session.ExpiresAt.After(now) {
			delete(a.sessions, token)
		}
	}
}

func (a *SessionAuth) evictOldestLocked() {
	oldestToken := ""
	var oldest adminSession
	for token, session := range a.sessions {
		if oldestToken == "" || session.ExpiresAt.Before(oldest.ExpiresAt) || (session.ExpiresAt.Equal(oldest.ExpiresAt) && token < oldestToken) {
			oldestToken = token
			oldest = session
		}
	}
	if oldestToken != "" {
		delete(a.sessions, oldestToken)
	}
}

func (a *SessionAuth) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookie,
		Value:    "",
		Path:     "/admin",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (a *SessionAuth) renderLogin(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = web.RenderLogin(context.Background(), w, web.LoginData{Message: message})
}

func setAdminSecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", adminContentSecurityPolicy)
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}

func parseAdminForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "admin request body too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid admin form", http.StatusBadRequest)
		}
		return false
	}
	return true
}

func csrfEqual(r *http.Request, expected string) bool {
	got := r.Header.Get("X-CSRF-Token")
	if got == "" {
		got = r.FormValue("csrf_token")
	}
	if got == "" && r.URL.Query().Has("csrf_token") {
		got = r.URL.Query().Get("csrf_token")
	}
	if got == "" && r.Body != nil {
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxAdminFormBodyBytes))
		if err == nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			vals, err := url.ParseQuery(string(bodyBytes))
			if err == nil {
				got = vals.Get("csrf_token")
			}
		}
	}
	return secretEqual(got, expected)
}

func secretEqual(got, expected string) bool {
	if got == "" || expected == "" {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) == 1
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = r.Header.Get("X-Forwarded-Scheme")
	}
	if proto != "" {
		if first, _, ok := strings.Cut(proto, ","); ok {
			proto = first
		}
		return strings.EqualFold(strings.TrimSpace(proto), "https")
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Ssl"), "on") {
		return true
	}
	if forwarded := r.Header.Get("Forwarded"); forwarded != "" {
		for _, part := range strings.Split(forwarded, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok && strings.EqualFold(k, "proto") && strings.EqualFold(strings.Trim(v, `"`), "https") {
				return true
			}
		}
	}
	return false
}
