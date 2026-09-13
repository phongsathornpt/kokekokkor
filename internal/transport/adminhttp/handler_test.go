package adminhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	appoauth "github.com/phongsathornpt/kokekokkor/internal/application/oauth"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type fakeTokens struct {
	values  map[string]domainoauth.TokenSet
	deleted string
}

func (f *fakeTokens) Get(_ context.Context, providerID string) (domainoauth.TokenSet, error) {
	value, ok := f.values[providerID]
	if !ok {
		return domainoauth.TokenSet{}, appoauth.ErrTokenNotFound
	}
	return value, nil
}

func (f *fakeTokens) Delete(_ context.Context, providerID string) error {
	if _, ok := f.values[providerID]; !ok {
		return errors.New("unexpected provider")
	}
	delete(f.values, providerID)
	f.deleted = providerID
	return nil
}

type fakeCredentials struct {
	values   map[string]string
	editable bool
}

func (f *fakeCredentials) HasAPIKey(providerID string) bool { return f.values[providerID] != "" }
func (f *fakeCredentials) Editable() bool                   { return f.editable }
func (f *fakeCredentials) SetAPIKey(_ context.Context, providerID, value string) error {
	if !f.editable {
		return errors.New("read only")
	}
	f.values[providerID] = value
	return nil
}
func (f *fakeCredentials) DeleteAPIKey(_ context.Context, providerID string) error {
	if !f.editable {
		return errors.New("read only")
	}
	delete(f.values, providerID)
	return nil
}

func testSnapshot() domaincatalog.Snapshot {
	return domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "gemini", Protocol: provider.ProtocolGemini, BaseURL: "https://generativelanguage.googleapis.com", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolGemini: "gemini"},
		Routes:    map[string][]domaincatalog.RouteTarget{"portable": {{ProviderID: "gemini", Model: "gemini-2.5-pro"}}},
	}
}

func TestAdminRendersStatusWithoutSecrets(t *testing.T) {
	tokens := &fakeTokens{values: map[string]domainoauth.TokenSet{"gemini": {AccessToken: "super-secret-access", RefreshToken: "super-secret-refresh"}}}
	handler, err := New(testSnapshot(), tokens, []string{"gemini"}, map[string]bool{"gemini": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, want := range []string{"gemini", "connected", "configured", "portable", "gemini-2.5-pro"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	for _, secret := range []string{"super-secret-access", "super-secret-refresh"} {
		if strings.Contains(body, secret) {
			t.Fatalf("body leaked secret %q", secret)
		}
	}
}

func TestAdminDisconnectsOAuth(t *testing.T) {
	tokens := &fakeTokens{values: map[string]domainoauth.TokenSet{"gemini": {AccessToken: "access"}}}
	handler, err := New(testSnapshot(), tokens, []string{"gemini"}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodDelete, "/admin/oauth/gemini", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || tokens.deleted != "gemini" {
		t.Fatalf("status=%d deleted=%q", rec.Code, tokens.deleted)
	}
	if rec.Header().Get("HX-Refresh") != "true" {
		t.Fatalf("HX-Refresh = %q", rec.Header().Get("HX-Refresh"))
	}
}

func TestAdminUpdatesAndDeletesEncryptedAPIKey(t *testing.T) {
	credentials := &fakeCredentials{values: map[string]string{}, editable: true}
	handler, err := NewManageable(testSnapshot(), nil, credentials, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}

	form := url.Values{"provider_id": {"gemini"}, "api_key": {"new-secret"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/credentials/api-key", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || credentials.values["gemini"] != "new-secret" {
		t.Fatalf("set status=%d values=%#v", rec.Code, credentials.values)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "configured") || !strings.Contains(body, "Save key") {
		t.Fatalf("body missing credential controls: %s", body)
	}
	if strings.Contains(body, "new-secret") {
		t.Fatal("admin page leaked API key")
	}

	form = url.Values{"provider_id": {"gemini"}}
	req = httptest.NewRequest(http.MethodPost, "/admin/credentials/api-key/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || credentials.HasAPIKey("gemini") {
		t.Fatalf("delete status=%d values=%#v", rec.Code, credentials.values)
	}
}
