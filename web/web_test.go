package web_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/web"
)

func sampleData() web.DashboardData {
	return web.DashboardData{
		Providers: []web.ProviderView{
			{
				ID:             "openai-main",
				Protocol:       "openai",
				BaseURL:        "https://api.openai.com",
				Enabled:        true,
				OAuthAvailable: false,
				OAuthConnected: false,
				APIKey:         true,
			},
			{
				ID:             "anthropic-backup",
				Protocol:       "anthropic",
				BaseURL:        "https://api.anthropic.com",
				Enabled:        false,
				OAuthAvailable: false,
				OAuthConnected: false,
				APIKey:         false,
			},
			{
				ID:             "gemini-oauth",
				Protocol:       "gemini",
				BaseURL:        "https://generativelanguage.googleapis.com",
				Enabled:        true,
				OAuthAvailable: true,
				OAuthConnected: true,
				APIKey:         false,
			},
		},
		Defaults: []web.DefaultView{
			{
				Protocol:          "openai",
				ProviderID:        "openai-main",
				EligibleProviders: []string{"openai-main"},
			},
			{
				Protocol:          "gemini",
				ProviderID:        "gemini-oauth",
				EligibleProviders: []string{"gemini-oauth"},
			},
		},
		Routes: []web.RouteView{
			{
				Model:      "fast-chat",
				Targets:    []string{"openai-main → gpt-4o-mini", "gemini-oauth → gemini-1.5-flash"},
				TargetSpec: "openai-main:gpt-4o-mini, gemini-oauth:gemini-1.5-flash",
			},
		},
		CSRF:               "csrf-test-token-12345",
		Editable:           true,
		CredentialEditable: true,
		Protocols:          []string{"openai", "anthropic", "gemini"},
	}
}

func TestRenderOverview(t *testing.T) {
	data := sampleData()
	var buf bytes.Buffer
	if err := web.RenderOverview(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderOverview() error = %v", err)
	}
	output := buf.String()
	expected := []string{
		"Gateway administration",
		"csrf-test-token-12345",
		"openai-main",
		"anthropic-backup",
		"gemini-oauth",
		"fast-chat",
		`id="alert-banner"`,
		"Upstream providers",
		"Active routes",
		"Protocol defaults",
		"href=\"/admin/providers\"",
		"href=\"/admin/defaults\"",
		"href=\"/admin/routes\"",
		"<style>",
		"</style>",
		"--color-ground",
	}
	for _, substr := range expected {
		if !strings.Contains(output, substr) {
			t.Errorf("rendered overview missing expected substring: %q", substr)
		}
	}
}

func TestRenderProviders(t *testing.T) {
	data := sampleData()
	var buf bytes.Buffer
	if err := web.RenderProviders(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderProviders() error = %v", err)
	}
	output := buf.String()
	expected := []string{
		"Gateway administration",
		"csrf-test-token-12345",
		"openai-main",
		"anthropic-backup",
		"gemini-oauth",
		"Save key",
		"Add provider",
		"Save provider",
		"Delete provider",
		`id="alert-banner"`,
		`id="add-provider-dialog"`,
		`id="add-provider-alert"`,
		`<label for="new-protocol">Protocol</label>`,
		`id="new-protocol" name="protocol" required`,
		`<label for="new-base-url">Base URL</label>`,
		`<label for="new-api-key">API Key (optional)</label>`,
		`hx-post="/admin/providers/toggle"`,
		`hx-delete="/admin/oauth/gemini-oauth"`,
		`id="oauth-modal-gemini-oauth"`,
		"Manage OAuth",
		"Direct Token Import",
		`hx-include="closest form"`,
		`hx-boost:inherited="true"`,
		`htmx.org@4.0.0`,
	}
	for _, substr := range expected {
		if !strings.Contains(output, substr) {
			t.Errorf("rendered providers missing expected substring: %q", substr)
		}
	}
	if strings.Contains(output, "responseHandling") {
		t.Error("rendered providers still contains removed htmx v2 responseHandling config")
	}
}

func TestRenderDefaults(t *testing.T) {
	data := sampleData()
	var buf bytes.Buffer
	if err := web.RenderDefaults(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderDefaults() error = %v", err)
	}
	output := buf.String()
	expected := []string{
		"Gateway administration",
		"csrf-test-token-12345",
		"Protocol defaults",
		`<label for="default-select-openai">Default provider</label>`,
		`<label for="default-select-gemini">Default provider</label>`,
		`hx-post="/admin/defaults"`,
	}
	for _, substr := range expected {
		if !strings.Contains(output, substr) {
			t.Errorf("rendered defaults missing expected substring: %q", substr)
		}
	}
}

func TestRenderRoutes(t *testing.T) {
	data := sampleData()
	var buf bytes.Buffer
	if err := web.RenderRoutes(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderRoutes() error = %v", err)
	}
	output := buf.String()
	expected := []string{
		"Gateway administration",
		"csrf-test-token-12345",
		"Model routes",
		"fast-chat",
		"Fallback chain",
		"Add route",
		`id="add-route-dialog"`,
		`id="add-route-alert"`,
		`hx-post="/admin/routes"`,
		`hx-post="/admin/routes/delete"`,
	}
	for _, substr := range expected {
		if !strings.Contains(output, substr) {
			t.Errorf("rendered routes missing expected substring: %q", substr)
		}
	}
}

func TestRenderDashboard(t *testing.T) {
	pages := []struct {
		page     web.PageName
		expected string
	}{
		{web.PageOverview, "Upstream providers"},
		{web.PageProviders, "Save provider"},
		{web.PageDefaults, "Protocol defaults"},
		{web.PageRoutes, "Model routes"},
	}
	for _, p := range pages {
		data := sampleData()
		data.ActivePage = p.page
		var buf bytes.Buffer
		if err := web.RenderDashboard(context.Background(), &buf, data); err != nil {
			t.Fatalf("RenderDashboard(%s) error = %v", p.page, err)
		}
		if !strings.Contains(buf.String(), p.expected) {
			t.Errorf("RenderDashboard(%s) missing %q", p.page, p.expected)
		}
	}
}

func TestAdminSidebarLayoutOnly(t *testing.T) {
	pages := []struct {
		name string
		fn   func(context.Context, io.Writer, web.DashboardData) error
	}{
		{"Overview", web.RenderOverview},
		{"Providers", web.RenderProviders},
		{"Defaults", web.RenderDefaults},
		{"Routes", web.RenderRoutes},
	}

	for _, tc := range pages {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			data := sampleData()
			if err := tc.fn(context.Background(), &buf, data); err != nil {
				t.Fatalf("%s render error: %v", tc.name, err)
			}
			out := buf.String()

			// Must contain sidebar element
			if !strings.Contains(out, "<aside") {
				t.Errorf("%s missing sidebar <aside> tag", tc.name)
			}

			// Sidebar must contain primary navigation links
			for _, href := range []string{`href="/admin"`, `href="/admin/providers"`, `href="/admin/defaults"`, `href="/admin/routes"`} {
				if !strings.Contains(out, href) {
					t.Errorf("%s missing sidebar navigation link %s", tc.name, href)
				}
			}

			// Must not contain top navigation header
			if strings.Contains(out, "<header") {
				t.Errorf("%s must not contain a <header> top rail; menu must be sidebar-only", tc.name)
			}
		})
	}
}

func TestRenderLogin(t *testing.T) {
	tests := []struct {
		name        string
		data        web.LoginData
		wantMessage string
	}{
		{
			name:        "clean login form",
			data:        web.LoginData{},
			wantMessage: "",
		},
		{
			name: "login form with error alert",
			data: web.LoginData{
				Message: "invalid credentials provided",
			},
			wantMessage: "invalid credentials provided",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := web.RenderLogin(context.Background(), &buf, tc.data)
			if err != nil {
				t.Fatalf("RenderLogin() returned error: %v", err)
			}

			output := buf.String()

			expectedSubstrings := []string{
				"<title>kokekokkor admin login</title>",
				`<form method="post" action="/admin/login">`,
				`type="password"`,
				`id="password"`,
				`name="password"`,
				"Gateway administration",
				"AES-256 encrypted credential store",
				"<style>",
				"</style>",
			}

			for _, substr := range expectedSubstrings {
				if !strings.Contains(output, substr) {
					t.Errorf("rendered login missing expected substring: %q", substr)
				}
			}

			if tc.wantMessage != "" {
				if !strings.Contains(output, tc.wantMessage) {
					t.Errorf("rendered login missing error alert %q", tc.wantMessage)
				}
			} else {
				if strings.Contains(output, `class="error`) {
					t.Error("rendered login unexpectedly contains error alert div")
				}
			}
		})
	}
}

func TestRenderProviderModelsAndTestButton(t *testing.T) {
	data := sampleData()
	data.Providers = append(data.Providers, web.ProviderView{
		ID:       "antigravity",
		Protocol: "gemini",
		BaseURL:  "https://cloudcode-pa.googleapis.com",
		Enabled:  true,
		Models: []web.ModelView{
			{
				ID:           "claude-opus-4-6-thinking",
				Name:         "Claude Opus 4.6 (Thinking)",
				Capabilities: []string{"thinking", "code", "chat"},
				IsDefault:    true,
			},
			{
				ID:            "gemini-3.8-flash-high",
				Name:          "Gemini 3.8 Flash (High)",
				UpstreamModel: "gemini-3.8-flash-high(high)",
				Capabilities:  []string{"thinking", "fast"},
			},
		},
	})

	var buf bytes.Buffer
	if err := web.RenderProviders(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderProviders() error = %v", err)
	}
	output := buf.String()
	expected := []string{
		"Supported Models (2)",
		"claude-opus-4-6-thinking",
		"Claude Opus 4.6 (Thinking)",
		"gemini-3.8-flash-high",
		`hx-post="/admin/providers/antigravity/test-model"`,
		"DEFAULT",
		"thinking",
		`data-copy="antigravity/claude-opus-4-6-thinking"`,
	}
	for _, substr := range expected {
		if !strings.Contains(output, substr) {
			t.Errorf("rendered providers missing expected substring: %q", substr)
		}
	}
}
