package adminhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
