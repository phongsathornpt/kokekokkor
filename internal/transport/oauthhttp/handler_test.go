package oauthhttp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

type fakeService struct {
	beginRedirect string
	beginURI      string
	beginErr      error
	completeState string
	completeCode  string
	completeURI   string
	completeErr   error
}

func (f *fakeService) Begin(_ context.Context, _ domainoauth.Provider, redirectURI string) (domainoauth.Authorization, error) {
	f.beginURI = redirectURI
	if f.beginErr != nil {
		return domainoauth.Authorization{}, f.beginErr
	}
	return domainoauth.Authorization{URL: f.beginRedirect}, nil
}

func (f *fakeService) Complete(_ context.Context, _ domainoauth.Provider, state, code, redirectURI string) (domainoauth.TokenSet, error) {
	f.completeState = state
	f.completeCode = code
	f.completeURI = redirectURI
	if f.completeErr != nil {
		return domainoauth.TokenSet{}, f.completeErr
	}
	return domainoauth.TokenSet{AccessToken: "must-not-leak"}, nil
}

func TestHandlerStartAndCallback(t *testing.T) {
	service := &fakeService{beginRedirect: "https://accounts.example.com/authorize?state=abc"}
	handler, err := New(service, map[string]domainoauth.Provider{
		"gemini": {ID: "gemini", AuthorizationURL: "https://accounts.example.com/authorize", TokenURL: "https://accounts.example.com/token", ClientID: "client-id"},
	}, "https://gateway.example.com")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /oauth/{provider}/start", handler)
	mux.Handle("GET /oauth/{provider}/callback", handler)

	start := httptest.NewRecorder()
	mux.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/oauth/gemini/start", nil))
	if start.Code != http.StatusFound || start.Header().Get("Location") != service.beginRedirect {
		t.Fatalf("start status=%d location=%q", start.Code, start.Header().Get("Location"))
	}
	if service.beginURI != "https://gateway.example.com/oauth/gemini/callback" {
		t.Fatalf("begin redirect URI = %q", service.beginURI)
	}

	callback := httptest.NewRecorder()
	mux.ServeHTTP(callback, httptest.NewRequest(http.MethodGet, "/oauth/gemini/callback?state=state-1&code=code-1", nil))
	if callback.Code != http.StatusOK {
		t.Fatalf("callback status=%d body=%s", callback.Code, callback.Body.String())
	}
	if service.completeState != "state-1" || service.completeCode != "code-1" || service.completeURI != service.beginURI {
		t.Fatalf("complete state=%q code=%q uri=%q", service.completeState, service.completeCode, service.completeURI)
	}
	if strings.Contains(callback.Body.String(), "must-not-leak") {
		t.Fatalf("callback leaked token material: %s", callback.Body.String())
	}
}

func TestHandlerRedactsServiceErrorsAndLogsDetails(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		service    *fakeService
		publicText string
		operation  string
	}{
		{
			name:       "start",
			path:       "/oauth/gemini/start",
			service:    &fakeService{beginErr: errors.New("state repository secret=alpha")},
			publicText: "OAuth authorization could not be started",
			operation:  "start",
		},
		{
			name:       "callback",
			path:       "/oauth/gemini/callback?state=s&code=c",
			service:    &fakeService{completeErr: errors.New("credential store secret=beta")},
			publicText: "OAuth authorization could not be completed",
			operation:  "callback",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			handler, err := NewWithLogger(tc.service, map[string]domainoauth.Provider{
				"gemini": {ID: "gemini", AuthorizationURL: "https://accounts.example.com/authorize", TokenURL: "https://accounts.example.com/token", ClientID: "client-id"},
			}, "https://gateway.example.com", logger)
			if err != nil {
				t.Fatalf("NewWithLogger() error = %v", err)
			}
			mux := http.NewServeMux()
			mux.Handle("GET /oauth/{provider}/start", handler)
			mux.Handle("GET /oauth/{provider}/callback", handler)

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("X-Request-ID", "req-123")
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d", response.Code)
			}
			if !strings.Contains(response.Body.String(), tc.publicText) {
				t.Fatalf("body = %s", response.Body.String())
			}
			if strings.Contains(response.Body.String(), "secret=") {
				t.Fatalf("public response leaked internal error: %s", response.Body.String())
			}
			logText := logs.String()
			for _, want := range []string{"req-123", "gemini", tc.operation, "secret="} {
				if !strings.Contains(logText, want) {
					t.Fatalf("logs %q missing %q", logText, want)
				}
			}
		})
	}
}

func TestHandlerRejectsUnknownProvider(t *testing.T) {
	handler, err := New(&fakeService{}, map[string]domainoauth.Provider{}, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /oauth/{provider}/start", handler)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/oauth/missing/start", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestHandlerRejectsInsecurePublicBaseURL(t *testing.T) {
	_, err := New(&fakeService{}, nil, "http://gateway.example.com")
	if err == nil {
		t.Fatal("New() error = nil, want HTTPS validation error")
	}
}

func TestCallbackURLPathEscapesProviderID(t *testing.T) {
	handler, err := New(&fakeService{}, nil, "https://gateway.example.com/")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	got := handler.callbackURL("provider/a")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.EscapedPath() != "/oauth/provider%2Fa/callback" {
		t.Fatalf("escaped path = %q", parsed.EscapedPath())
	}
}
