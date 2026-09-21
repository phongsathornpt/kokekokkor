package web_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/web"
)

func TestRenderDashboard(t *testing.T) {
	data := web.DashboardData{
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

	var buf bytes.Buffer
	err := web.RenderDashboard(context.Background(), &buf, data)
	if err != nil {
		t.Fatalf("RenderDashboard() returned error: %v", err)
	}

	output := buf.String()

	// Verify crucial HTML structure and test-critical strings
	expectedSubstrings := []string{
		"Gateway administration",
		"csrf-test-token-12345",
		"openai-main",
		"anthropic-backup",
		"gemini-oauth",
		"fast-chat",
		"Save key",
		"Add provider",
		"Save provider",
		"Delete provider",
		`id="alert-banner"`,
		`<label for="new-protocol">Protocol</label>`,
		`<select id="new-protocol" name="protocol" required>`,
		`<label for="new-base-url">Base URL</label>`,
		`<label for="default-select-gemini">Default provider</label>`,
		`hx-post="/admin/providers/toggle"`,
		`hx-post="/admin/defaults"`,
		`hx-post="/admin/routes"`,
		`hx-delete="/admin/oauth/gemini-oauth"`,
		"<style>",
		"</style>",
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(output, substr) {
			t.Errorf("rendered dashboard missing expected substring: %q", substr)
		}
	}

	// Verify embedded CSS has been injected inside <style>
	if !strings.Contains(output, "#090a0f") {
		t.Error("rendered dashboard does not contain expected dark theme CSS tokens")
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
