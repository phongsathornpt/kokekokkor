package adminhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
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
	values    map[string]string
	editable  bool
	forgotten string
}

func (f *fakeCredentials) HasAPIKey(providerID string) bool { return f.values[providerID] != "" }
func (f *fakeCredentials) Editable() bool                   { return f.editable }
func (f *fakeCredentials) ForgetProvider(providerID string) {
	delete(f.values, providerID)
	f.forgotten = providerID
}
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

type fakeCatalog struct {
	snapshot domaincatalog.Snapshot
}

func (f *fakeCatalog) Load(context.Context) (domaincatalog.Snapshot, error) { return f.snapshot, nil }
func (f *fakeCatalog) CreateProvider(_ context.Context, item domaincatalog.Provider) error {
	f.snapshot.Providers = append(f.snapshot.Providers, item)
	return nil
}
func (f *fakeCatalog) UpdateProvider(_ context.Context, providerID string, item domaincatalog.Provider) error {
	for i := range f.snapshot.Providers {
		if f.snapshot.Providers[i].ID == providerID {
			f.snapshot.Providers[i] = item
			return nil
		}
	}
	return errors.New("missing provider")
}
func (f *fakeCatalog) DeleteProvider(_ context.Context, providerID string) error {
	for i := range f.snapshot.Providers {
		if f.snapshot.Providers[i].ID == providerID {
			f.snapshot.Providers = append(f.snapshot.Providers[:i], f.snapshot.Providers[i+1:]...)
			return nil
		}
	}
	return errors.New("missing provider")
}
func (f *fakeCatalog) SetProviderEnabled(_ context.Context, providerID string, enabled bool) error {
	for i := range f.snapshot.Providers {
		if f.snapshot.Providers[i].ID == providerID {
			f.snapshot.Providers[i].Enabled = enabled
			return nil
		}
	}
	return errors.New("missing provider")
}
func (f *fakeCatalog) SetDefault(_ context.Context, protocolName provider.Protocol, providerID string) error {
	if providerID == "" {
		delete(f.snapshot.Defaults, protocolName)
	} else {
		f.snapshot.Defaults[protocolName] = providerID
	}
	return nil
}
func (f *fakeCatalog) SetRoute(_ context.Context, model string, targets []domaincatalog.RouteTarget) error {
	f.snapshot.Routes[model] = targets
	return nil
}
func (f *fakeCatalog) DeleteRoute(_ context.Context, model string) error {
	delete(f.snapshot.Routes, model)
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

func TestAdminProviderCRUDAndCredentialForget(t *testing.T) {
	catalog := &fakeCatalog{snapshot: domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{},
		Defaults:  map[provider.Protocol]string{},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}}
	credentials := &fakeCredentials{values: map[string]string{"custom": "stale"}, editable: true}
	handler, err := NewManageable(catalog.snapshot, catalog, credentials, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}

	form := url.Values{
		"provider_id": {"custom"},
		"protocol":    {"openai"},
		"base_url":    {"https://one.example/v1"},
		"enabled":     {"true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || len(catalog.snapshot.Providers) != 1 {
		t.Fatalf("create status=%d providers=%#v", rec.Code, catalog.snapshot.Providers)
	}

	form.Set("protocol", "anthropic")
	form.Set("base_url", "https://api.anthropic.com")
	form.Set("enabled", "false")
	req = httptest.NewRequest(http.MethodPost, "/admin/providers/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	got := catalog.snapshot.Providers[0]
	if rec.Code != http.StatusNoContent || got.Protocol != provider.ProtocolAnthropic || got.BaseURL != "https://api.anthropic.com" || got.Enabled {
		t.Fatalf("update status=%d provider=%#v", rec.Code, got)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	for _, want := range []string{"Add provider", "Save provider", "Delete provider"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("admin body missing %q", want)
		}
	}

	form = url.Values{"provider_id": {"custom"}}
	req = httptest.NewRequest(http.MethodPost, "/admin/providers/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || len(catalog.snapshot.Providers) != 0 {
		t.Fatalf("delete status=%d providers=%#v", rec.Code, catalog.snapshot.Providers)
	}
	if credentials.forgotten != "custom" || credentials.HasAPIKey("custom") {
		t.Fatalf("credential forget failed: forgotten=%q values=%#v", credentials.forgotten, credentials.values)
	}
}

func TestAdminProviderRejectsInvalidProtocol(t *testing.T) {
	catalog := &fakeCatalog{snapshot: domaincatalog.Snapshot{Defaults: map[provider.Protocol]string{}, Routes: map[string][]domaincatalog.RouteTarget{}}}
	handler, err := NewManageable(catalog.snapshot, catalog, &fakeCredentials{values: map[string]string{}}, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}
	form := url.Values{"provider_id": {"bad"}, "protocol": {"wat"}, "base_url": {"https://example.com"}, "enabled": {"true"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminToggleProvider(t *testing.T) {
	catalog := &fakeCatalog{snapshot: domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "p1", Protocol: provider.ProtocolOpenAI, BaseURL: "https://api.openai.com", Enabled: true}},
		Defaults:  map[provider.Protocol]string{},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}}
	handler, err := NewManageable(catalog.snapshot, catalog, &fakeCredentials{values: map[string]string{}}, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}

	form := url.Values{"provider_id": {"p1"}, "enabled": {"false"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/providers/toggle", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || catalog.snapshot.Providers[0].Enabled {
		t.Fatalf("toggle status=%d enabled=%v", rec.Code, catalog.snapshot.Providers[0].Enabled)
	}
	if rec.Header().Get("HX-Refresh") != "true" {
		t.Fatalf("HX-Refresh = %q", rec.Header().Get("HX-Refresh"))
	}
}

func TestAdminRendersAccessibleFormControlsAndAlertBanner(t *testing.T) {
	catalog := &fakeCatalog{snapshot: testSnapshot()}
	handler, err := NewEditable(catalog.snapshot, catalog, &fakeTokens{values: map[string]domainoauth.TokenSet{}}, []string{"gemini"}, nil)
	if err != nil {
		t.Fatalf("NewEditable() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	for _, want := range []string{
		`id="alert-banner"`,
		`<label for="new-protocol">Protocol</label>`,
		`<select id="new-protocol" name="protocol" required>`,
		`<label for="new-base-url">Base URL</label>`,
		`<label for="default-select-gemini">Default provider</label>`,
		`<select id="default-select-gemini"`,
		`<option value="gemini" selected>gemini</option>`,
		`hx-post="/admin/providers/toggle"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestAdminMutationErrorSetsRetargetHeaders(t *testing.T) {
	catalog := &fakeCatalog{snapshot: domaincatalog.Snapshot{Defaults: map[provider.Protocol]string{}, Routes: map[string][]domaincatalog.RouteTarget{}}}
	handler, err := NewManageable(catalog.snapshot, catalog, &fakeCredentials{values: map[string]string{}}, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}
	form := url.Values{"provider_id": {"bad"}, "protocol": {"invalid-protocol"}, "base_url": {"https://example.com"}, "enabled": {"true"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("HX-Retarget") != "#alert-banner" {
		t.Fatalf("HX-Retarget = %q, want #alert-banner", rec.Header().Get("HX-Retarget"))
	}
	if rec.Header().Get("HX-Reswap") != "innerHTML" {
		t.Fatalf("HX-Reswap = %q, want innerHTML", rec.Header().Get("HX-Reswap"))
	}
}
