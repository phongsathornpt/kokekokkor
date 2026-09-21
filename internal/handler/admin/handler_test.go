package adminhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/usecase/probing"
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

func (f *fakeTokens) Put(_ context.Context, providerID string, tokens domainoauth.TokenSet) error {
	if f.values == nil {
		f.values = make(map[string]domainoauth.TokenSet)
	}
	f.values[providerID] = tokens
	return nil
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
	failSet   bool
}

func (f *fakeCredentials) HasAPIKey(providerID string) bool   { return f.values[providerID] != "" }
func (f *fakeCredentials) GetAPIKey(providerID string) string { return f.values[providerID] }
func (f *fakeCredentials) Editable() bool                     { return f.editable }
func (f *fakeCredentials) ForgetProvider(providerID string) {
	delete(f.values, providerID)
	f.forgotten = providerID
}
func (f *fakeCredentials) SetAPIKey(_ context.Context, providerID, value string) error {
	if !f.editable {
		return errors.New("read only")
	}
	if f.failSet {
		return errors.New("simulated keyring encryption failure")
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

	req = httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
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

	req = httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
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

func TestAdminCreateProviderWithAPIKey(t *testing.T) {
	catalog := &fakeCatalog{snapshot: domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{},
		Defaults:  map[provider.Protocol]string{},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}}
	credentials := &fakeCredentials{values: map[string]string{}, editable: true}
	handler, err := NewManageable(catalog.snapshot, catalog, credentials, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}

	form := url.Values{
		"provider_id": {"openai-with-key"},
		"protocol":    {"openai"},
		"base_url":    {"https://api.openai.com/v1"},
		"enabled":     {"true"},
		"api_key":     {"sk-secret-token-12345"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("create with api_key status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(catalog.snapshot.Providers) != 1 {
		t.Fatalf("provider count = %d, want 1", len(catalog.snapshot.Providers))
	}
	if credentials.values["openai-with-key"] != "sk-secret-token-12345" {
		t.Fatalf("credential key = %q, want %q", credentials.values["openai-with-key"], "sk-secret-token-12345")
	}
}

func TestAdminCreateProviderAPIKeyFailureRollsBack(t *testing.T) {
	catalog := &fakeCatalog{snapshot: domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{},
		Defaults:  map[provider.Protocol]string{},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}}
	credentials := &fakeCredentials{values: map[string]string{}, editable: true, failSet: true}
	handler, err := NewManageable(catalog.snapshot, catalog, credentials, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}

	form := url.Values{
		"provider_id": {"openai-fail-key"},
		"protocol":    {"openai"},
		"base_url":    {"https://api.openai.com/v1"},
		"enabled":     {"true"},
		"api_key":     {"sk-secret-token-12345"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if len(catalog.snapshot.Providers) != 0 {
		t.Fatalf("provider count = %d, want 0 after rollback", len(catalog.snapshot.Providers))
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

	// 1. Overview page
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="alert-banner"`,
		`Upstream providers`,
		`Active routes`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("overview body missing %q", want)
		}
	}

	// 2. Providers page
	req = httptest.NewRequest(http.MethodGet, "/admin/providers", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/providers status = %d", rec.Code)
	}
	body = rec.Body.String()
	for _, want := range []string{
		`id="alert-banner"`,
		`<label for="new-protocol">Protocol</label>`,
		`id="new-protocol" name="protocol" required`,
		`<label for="new-base-url">Base URL</label>`,
		`hx-post="/admin/providers/toggle"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("providers body missing %q", want)
		}
	}

	// 3. Defaults page
	req = httptest.NewRequest(http.MethodGet, "/admin/defaults", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/defaults status = %d", rec.Code)
	}
	body = rec.Body.String()
	for _, want := range []string{
		`id="alert-banner"`,
		`<label for="default-select-gemini">Default provider</label>`,
		`<select id="default-select-gemini"`,
		`<option value="gemini" selected>gemini</option>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("defaults body missing %q", want)
		}
	}

	// 4. Routes page
	req = httptest.NewRequest(http.MethodGet, "/admin/routes", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/routes status = %d", rec.Code)
	}
	body = rec.Body.String()
	for _, want := range []string{
		`id="alert-banner"`,
		`<label for="new-route-model"`,
		`<label for="new-route-targets"`,
		`hx-post="/admin/routes"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("routes body missing %q", want)
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

func TestAdminDedicatedPageRoutes(t *testing.T) {
	catalog := &fakeCatalog{snapshot: testSnapshot()}
	handler, err := NewEditable(catalog.snapshot, catalog, &fakeTokens{values: map[string]domainoauth.TokenSet{}}, []string{"gemini"}, nil)
	if err != nil {
		t.Fatalf("NewEditable() error = %v", err)
	}

	tests := []struct {
		name      string
		path      string
		wantTitle string
	}{
		{name: "root admin", path: "/admin", wantTitle: "kokekokkor admin - overview"},
		{name: "admin overview", path: "/admin/overview", wantTitle: "kokekokkor admin - overview"},
		{name: "admin providers", path: "/admin/providers", wantTitle: "kokekokkor admin - providers"},
		{name: "admin defaults", path: "/admin/defaults", wantTitle: "kokekokkor admin - protocol defaults"},
		{name: "admin routes", path: "/admin/routes", wantTitle: "kokekokkor admin - model routes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tc.path, rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.wantTitle) {
				t.Errorf("GET %s missing title %q", tc.path, tc.wantTitle)
			}
			if !strings.Contains(body, `hx-boost:inherited="true"`) {
				t.Errorf("GET %s missing hx-boost", tc.path)
			}
		})
	}
}

type fakeAdminOAuthService struct {
	deviceAuth  domainoauth.DeviceAuthorization
	deviceErr   error
	pollTokens  domainoauth.TokenSet
	pollErr     error
	exchangeErr error
}

func (f *fakeAdminOAuthService) DeviceAuthorize(context.Context, domainoauth.Provider) (domainoauth.DeviceAuthorization, error) {
	return f.deviceAuth, f.deviceErr
}

func (f *fakeAdminOAuthService) DevicePoll(context.Context, domainoauth.Provider, string) (domainoauth.TokenSet, error) {
	return f.pollTokens, f.pollErr
}

func (f *fakeAdminOAuthService) DirectExchange(context.Context, domainoauth.Provider, string, string) (domainoauth.TokenSet, error) {
	return f.pollTokens, f.exchangeErr
}

func (f *fakeAdminOAuthService) ImportToken(context.Context, string, domainoauth.TokenSet) error {
	return nil
}

func TestAdminOAuthDeviceCodeAndPoll(t *testing.T) {
	catalog := &fakeCatalog{snapshot: testSnapshot()}
	tokens := &fakeTokens{values: make(map[string]domainoauth.TokenSet)}
	handler, err := NewManageable(catalog.snapshot, catalog, &fakeCredentials{values: map[string]string{}}, tokens, []string{"github"})
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}
	oauthService := &fakeAdminOAuthService{
		deviceAuth: domainoauth.DeviceAuthorization{
			DeviceCode:              "dev-code-123",
			UserCode:                "WDJB-4321",
			VerificationURI:         "https://github.com/login/device",
			VerificationURIComplete: "https://github.com/login/device?user_code=WDJB-4321",
			Interval:                5,
		},
		pollTokens: domainoauth.TokenSet{AccessToken: "token-abc"},
	}
	handler.SetOAuthService(oauthService, map[string]domainoauth.Provider{
		"github": {ID: "github", FlowType: domainoauth.FlowTypeDeviceCode},
	})

	// 1. Request device code
	req := httptest.NewRequest(http.MethodPost, "/admin/oauth/github/device-code", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("device-code status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "WDJB-4321") || !strings.Contains(body, "https://github.com/login/device") {
		t.Fatalf("body missing expected user code: %s", body)
	}

	// 2. Poll device code
	req = httptest.NewRequest(http.MethodGet, "/admin/oauth/github/poll?device_code=dev-code-123", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("poll status = %d, want 204", rec.Code)
	}
}

func TestAdminOAuthManualExchangeAndImport(t *testing.T) {
	catalog := &fakeCatalog{snapshot: testSnapshot()}
	tokens := &fakeTokens{values: make(map[string]domainoauth.TokenSet)}
	handler, err := NewManageable(catalog.snapshot, catalog, &fakeCredentials{values: map[string]string{}}, tokens, []string{"anthropic"})
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}
	oauthService := &fakeAdminOAuthService{
		pollTokens: domainoauth.TokenSet{AccessToken: "exchanged-token"},
	}
	handler.SetOAuthService(oauthService, map[string]domainoauth.Provider{
		"anthropic": {ID: "anthropic", FlowType: domainoauth.FlowTypeAuthorizationCode},
	})

	// 1. Manual Exchange with callback URL
	form := url.Values{"code": {"https://localhost:8080/callback?code=test-code-123&state=xyz"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/oauth/anthropic/exchange", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("exchange status = %d, want 204", rec.Code)
	}

	// 2. Direct Import Token
	form = url.Values{"token": {"sk-ant-oauth-token-999"}}
	req = httptest.NewRequest(http.MethodPost, "/admin/oauth/anthropic/import", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("import status = %d, want 204", rec.Code)
	}
	if tokens.values["anthropic"].AccessToken != "sk-ant-oauth-token-999" {
		t.Fatalf("tokens = %#v", tokens.values)
	}
}

type fakeAdminProber struct {
	result     probing.TestResult
	lastTarget provider.Target
	lastModel  string
}

func (f *fakeAdminProber) TestModel(_ context.Context, target provider.Target, modelID string) probing.TestResult {
	f.lastTarget = target
	f.lastModel = modelID
	return f.result
}

func TestAdminTestModel(t *testing.T) {
	catalog := &fakeCatalog{snapshot: domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{
			{ID: "antigravity", Protocol: provider.ProtocolGemini, BaseURL: "https://cloudcode-pa.googleapis.com", Enabled: true},
		},
	}}
	creds := &fakeCredentials{values: map[string]string{"antigravity": "test-key-123"}}
	handler, err := NewManageable(catalog.snapshot, catalog, creds, nil, nil)
	if err != nil {
		t.Fatalf("NewManageable() error = %v", err)
	}

	prober := &fakeAdminProber{
		result: probing.TestResult{
			OK:         true,
			StatusCode: 200,
			Latency:    240 * time.Millisecond,
			LatencyMs:  240,
			Snippet:    "Hello from Antigravity",
		},
	}
	handler.SetProbingService(prober)

	// 1. HTMX request - success
	form := url.Values{"model": {"claude-opus-4-6-thinking"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/providers/antigravity/test-model", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "200 OK (240ms)") {
		t.Fatalf("body = %q, want 200 OK (240ms)", body)
	}
	if prober.lastModel != "claude-opus-4-6-thinking" {
		t.Errorf("lastModel = %q, want claude-opus-4-6-thinking", prober.lastModel)
	}
	if prober.lastTarget.APIKey != "test-key-123" {
		t.Errorf("lastTarget.APIKey = %q, want test-key-123", prober.lastTarget.APIKey)
	}

	// 2. JSON request - success
	req = httptest.NewRequest(http.MethodPost, "/admin/providers/antigravity/test-model", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var jsonRes probing.TestResult
	if err := json.Unmarshal(rec.Body.Bytes(), &jsonRes); err != nil {
		t.Fatalf("unmarshal json response: %v", err)
	}
	if !jsonRes.OK || jsonRes.StatusCode != 200 || jsonRes.LatencyMs != 240 {
		t.Fatalf("jsonRes = %#v", jsonRes)
	}

	// 3. HTMX request - failure
	prober.result = probing.TestResult{
		OK:           false,
		StatusCode:   429,
		Latency:      120 * time.Millisecond,
		LatencyMs:    120,
		ErrorMessage: "Quota exceeded",
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/providers/antigravity/test-model", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fragment rendered)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Failed (429)") {
		t.Fatalf("body = %q, want Failed (429)", rec.Body.String())
	}

	// 4. Missing model
	req = httptest.NewRequest(http.MethodPost, "/admin/providers/antigravity/test-model", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing model status = %d, want 400", rec.Code)
	}

	// 5. Unknown provider
	req = httptest.NewRequest(http.MethodPost, "/admin/providers/non-existent-xyz/test-model", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown provider status = %d, want 404", rec.Code)
	}
}
