package web_test

import (
	"bytes"
	"context"
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
		"Upstream Providers",
		"Active Model Routes",
		"Protocol Defaults",
		"href=\"/admin/providers\"",
		"href=\"/admin/defaults\"",
		"href=\"/admin/routes\"",
		"<style>",
		"</style>",
		"#090a0f",
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
		`<label for="new-protocol">Protocol</label>`,
		`<select id="new-protocol" name="protocol" required>`,
		`<label for="new-base-url">Base URL</label>`,
		`hx-post="/admin/providers/toggle"`,
		`hx-delete="/admin/oauth/gemini-oauth"`,
	}
	for _, substr := range expected {
		if !strings.Contains(output, substr) {
			t.Errorf("rendered providers missing expected substring: %q", substr)
		}
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
		"Protocol Defaults",
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
		"Model Routes",
		"fast-chat",
		"Fallback Chain",
		"Add route",
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
		{web.PageOverview, "Upstream Providers"},
		{web.PageProviders, "Save provider"},
		{web.PageDefaults, "Protocol Defaults"},
		{web.PageRoutes, "Model Routes"},
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
