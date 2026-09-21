package adminhttp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSessionAuthLoginAndCSRF(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AdminCSRFToken(r.Context()) == "" {
			t.Fatal("missing CSRF token in context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	auth := NewSessionAuth("secret", time.Hour, next)

	redirect := httptest.NewRecorder()
	auth.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "http://gateway/admin", nil))
	if redirect.Code != http.StatusSeeOther || redirect.Header().Get("Location") != "/admin/login" {
		t.Fatalf("redirect status=%d location=%q", redirect.Code, redirect.Header().Get("Location"))
	}

	form := url.Values{"password": {"secret"}}
	loginReq := httptest.NewRequest(http.MethodPost, "http://gateway/admin/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	login := httptest.NewRecorder()
	auth.ServeHTTP(login, loginReq)
	if login.Code != http.StatusSeeOther {
		t.Fatalf("login status=%d", login.Code)
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != adminSessionCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookies=%#v", cookies)
	}

	pageReq := httptest.NewRequest(http.MethodGet, "http://gateway/admin", nil)
	pageReq.AddCookie(cookies[0])
	page := httptest.NewRecorder()
	auth.ServeHTTP(page, pageReq)
	if page.Code != http.StatusNoContent {
		t.Fatalf("page status=%d", page.Code)
	}

	mutationReq := httptest.NewRequest(http.MethodPost, "http://gateway/admin/routes", nil)
	mutationReq.AddCookie(cookies[0])
	mutation := httptest.NewRecorder()
	auth.ServeHTTP(mutation, mutationReq)
	if mutation.Code != http.StatusForbidden {
		t.Fatalf("mutation status=%d", mutation.Code)
	}
}

func TestSessionAuthRejectsWrongPassword(t *testing.T) {
	auth := NewSessionAuth("secret", time.Hour, http.NotFoundHandler())
	form := url.Values{"password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "http://gateway/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	auth.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatal("wrong password issued a session cookie")
	}
}
