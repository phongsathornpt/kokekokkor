package oauth

import "testing"

func TestGenericProfile(t *testing.T) {
	profile, err := GenericProfile(ProfileOptions{
		ProviderID:       "provider-a",
		ClientID:         "client-id",
		AuthorizationURL: "https://login.example.com/authorize",
		TokenURL:         "https://login.example.com/token",
		Scopes:           []string{"scope:a", "scope:b"},
		AuthorizationParams: map[string]string{
			"audience": "example",
		},
	})
	if err != nil {
		t.Fatalf("GenericProfile() error = %v", err)
	}
	if profile.ID != "provider-a" || profile.ClientID != "client-id" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.AuthorizationParams["audience"] != "example" || len(profile.Scopes) != 2 {
		t.Fatalf("profile = %#v", profile)
	}
}

func TestGeminiProfile(t *testing.T) {
	profile, err := GeminiProfile(ProfileOptions{ClientID: "client-id"})
	if err != nil {
		t.Fatalf("GeminiProfile() error = %v", err)
	}
	if profile.ID != "gemini" || profile.AuthorizationURL != GoogleAuthorizationURL || profile.TokenURL != GoogleTokenURL {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.AuthorizationParams["access_type"] != "offline" || profile.AuthorizationParams["prompt"] != "consent" {
		t.Fatalf("AuthorizationParams = %#v", profile.AuthorizationParams)
	}
	if len(profile.Scopes) != 2 {
		t.Fatalf("Scopes = %#v", profile.Scopes)
	}
}

func TestGeminiProfileOverrides(t *testing.T) {
	profile, err := GeminiProfile(ProfileOptions{
		ProviderID:       "google-a",
		ClientID:         "client-id",
		AuthorizationURL: "https://login.example.com/authorize",
		TokenURL:         "https://login.example.com/token",
		Scopes:           []string{"custom"},
		AuthorizationParams: map[string]string{
			"prompt": "select_account",
		},
	})
	if err != nil {
		t.Fatalf("GeminiProfile() error = %v", err)
	}
	if profile.ID != "google-a" || profile.AuthorizationURL != "https://login.example.com/authorize" || profile.TokenURL != "https://login.example.com/token" {
		t.Fatalf("profile = %#v", profile)
	}
	if len(profile.Scopes) != 1 || profile.Scopes[0] != "custom" {
		t.Fatalf("Scopes = %#v", profile.Scopes)
	}
	if profile.AuthorizationParams["prompt"] != "select_account" || profile.AuthorizationParams["access_type"] != "offline" {
		t.Fatalf("AuthorizationParams = %#v", profile.AuthorizationParams)
	}
}

func TestProfileRequiresClientID(t *testing.T) {
	if _, err := GeminiProfile(ProfileOptions{}); err == nil {
		t.Fatal("GeminiProfile() error = nil, want client ID validation error")
	}
}

func TestGenericProfileRequiresHTTPS(t *testing.T) {
	if _, err := GenericProfile(ProfileOptions{
		ProviderID:       "provider-a",
		ClientID:         "client-id",
		AuthorizationURL: "http://login.example.com/authorize",
		TokenURL:         "https://login.example.com/token",
	}); err == nil {
		t.Fatal("GenericProfile() error = nil, want HTTPS validation error")
	}
}

func TestCodexProfile(t *testing.T) {
	profile, err := CodexProfile(ProfileOptions{})
	if err != nil {
		t.Fatalf("CodexProfile() error = %v", err)
	}
	if profile.ID != "openai" {
		t.Fatalf("profile.ID = %q, want %q", profile.ID, "openai")
	}
	if profile.ClientID != OpenAICodexClientID {
		t.Fatalf("profile.ClientID = %q, want %q", profile.ClientID, OpenAICodexClientID)
	}
	if profile.AuthorizationURL != OpenAIAuthorizationURL || profile.TokenURL != OpenAITokenURL {
		t.Fatalf("profile URLs = (%q, %q)", profile.AuthorizationURL, profile.TokenURL)
	}
	if len(profile.Scopes) != 5 {
		t.Fatalf("profile.Scopes = %#v, want 5 scopes", profile.Scopes)
	}
}

func TestCodexProfileOverrides(t *testing.T) {
	profile, err := CodexProfile(ProfileOptions{
		ProviderID:       "openai-custom",
		ClientID:         "custom-client-id",
		AuthorizationURL: "https://auth.custom.com/authorize",
		TokenURL:         "https://auth.custom.com/token",
		Scopes:           []string{"custom-scope"},
		AuthorizationParams: map[string]string{
			"prompt": "login",
		},
	})
	if err != nil {
		t.Fatalf("CodexProfile() error = %v", err)
	}
	if profile.ID != "openai-custom" || profile.ClientID != "custom-client-id" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.AuthorizationURL != "https://auth.custom.com/authorize" || profile.TokenURL != "https://auth.custom.com/token" {
		t.Fatalf("profile URLs = (%q, %q)", profile.AuthorizationURL, profile.TokenURL)
	}
	if len(profile.Scopes) != 1 || profile.Scopes[0] != "custom-scope" {
		t.Fatalf("Scopes = %#v", profile.Scopes)
	}
	if profile.AuthorizationParams["prompt"] != "login" {
		t.Fatalf("AuthorizationParams = %#v", profile.AuthorizationParams)
	}
}

func TestClaudeProfile(t *testing.T) {
	profile, err := ClaudeProfile(ProfileOptions{})
	if err != nil {
		t.Fatalf("ClaudeProfile() error = %v", err)
	}
	if profile.ID != "anthropic" {
		t.Fatalf("profile.ID = %q, want %q", profile.ID, "anthropic")
	}
	if profile.ClientID != ClaudeClientID {
		t.Fatalf("profile.ClientID = %q, want %q", profile.ClientID, ClaudeClientID)
	}
	if profile.AuthorizationURL != ClaudeAuthorizationURL || profile.TokenURL != ClaudeTokenURL {
		t.Fatalf("profile URLs = (%q, %q)", profile.AuthorizationURL, profile.TokenURL)
	}
	if len(profile.Scopes) != 2 {
		t.Fatalf("profile.Scopes = %#v, want 2 scopes", profile.Scopes)
	}
}

func TestGitHubCopilotProfile(t *testing.T) {
	profile, err := GitHubCopilotProfile(ProfileOptions{})
	if err != nil {
		t.Fatalf("GitHubCopilotProfile() error = %v", err)
	}
	if profile.ID != "github" {
		t.Fatalf("profile.ID = %q, want %q", profile.ID, "github")
	}
	if profile.FlowType != "device_code" {
		t.Fatalf("profile.FlowType = %q, want device_code", profile.FlowType)
	}
	if profile.ClientID != GitHubCopilotClientID {
		t.Fatalf("profile.ClientID = %q, want %q", profile.ClientID, GitHubCopilotClientID)
	}
	if profile.DeviceAuthorizationURL != GitHubDeviceAuthorizationURL || profile.TokenURL != GitHubTokenURL {
		t.Fatalf("profile URLs = (%q, %q)", profile.DeviceAuthorizationURL, profile.TokenURL)
	}
	if len(profile.Scopes) != 1 || profile.Scopes[0] != "read:user" {
		t.Fatalf("profile.Scopes = %#v, want read:user", profile.Scopes)
	}
}

func TestAntigravityProfile(t *testing.T) {
	profile, err := AntigravityProfile(ProfileOptions{ClientID: "test-client-id", ClientSecret: "test-client-secret"})
	if err != nil {
		t.Fatalf("AntigravityProfile() error = %v", err)
	}
	if profile.ID != "antigravity" {
		t.Fatalf("profile.ID = %q, want %q", profile.ID, "antigravity")
	}
	if profile.ClientID != "test-client-id" || profile.ClientSecret != "test-client-secret" {
		t.Fatalf("profile credentials = (%q, %q)", profile.ClientID, profile.ClientSecret)
	}
	if profile.AuthorizationURL != GoogleAuthorizationURL || profile.TokenURL != GoogleTokenURL {
		t.Fatalf("profile URLs = (%q, %q)", profile.AuthorizationURL, profile.TokenURL)
	}
	if len(profile.Scopes) != 3 {
		t.Fatalf("profile.Scopes = %#v, want 3 scopes", profile.Scopes)
	}
}

func TestChatGPTProfileAlias(t *testing.T) {
	profile, err := ChatGPTProfile(ProfileOptions{})
	if err != nil {
		t.Fatalf("ChatGPTProfile() error = %v", err)
	}
	if profile.ID != "openai" || profile.ClientID != OpenAICodexClientID {
		t.Fatalf("profile = %#v", profile)
	}
}
