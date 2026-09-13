package oauth

import "testing"

func TestOpenAIProfile(t *testing.T) {
	profile, err := OpenAIProfile(ProfileOptions{ClientID: "client-id"})
	if err != nil {
		t.Fatalf("OpenAIProfile() error = %v", err)
	}
	if profile.ID != "openai" || profile.AuthorizationURL != OpenAIAuthorizationURL || profile.TokenURL != OpenAITokenURL {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.ClientID != "client-id" {
		t.Fatalf("ClientID = %q", profile.ClientID)
	}
	if profile.AuthorizationParams["codex_cli_simplified_flow"] != "true" {
		t.Fatalf("AuthorizationParams = %#v", profile.AuthorizationParams)
	}
	if len(profile.Scopes) != 6 || profile.Scopes[3] != "offline_access" {
		t.Fatalf("Scopes = %#v", profile.Scopes)
	}
}

func TestAnthropicProfile(t *testing.T) {
	profile, err := AnthropicProfile(ProfileOptions{ProviderID: "claude", ClientID: "client-id"})
	if err != nil {
		t.Fatalf("AnthropicProfile() error = %v", err)
	}
	if profile.ID != "claude" || profile.AuthorizationURL != AnthropicAuthorizationURL || profile.TokenURL != AnthropicTokenURL {
		t.Fatalf("profile = %#v", profile)
	}
	if len(profile.Scopes) != 3 || profile.Scopes[1] != "user:inference" {
		t.Fatalf("Scopes = %#v", profile.Scopes)
	}
}

func TestGeminiProfile(t *testing.T) {
	profile, err := GeminiProfile(ProfileOptions{ClientID: "client-id"})
	if err != nil {
		t.Fatalf("GeminiProfile() error = %v", err)
	}
	if profile.AuthorizationURL != GoogleAuthorizationURL || profile.TokenURL != GoogleTokenURL {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.AuthorizationParams["access_type"] != "offline" || profile.AuthorizationParams["prompt"] != "consent" {
		t.Fatalf("AuthorizationParams = %#v", profile.AuthorizationParams)
	}
	if len(profile.Scopes) != 2 {
		t.Fatalf("Scopes = %#v", profile.Scopes)
	}
}

func TestProfileOverrides(t *testing.T) {
	profile, err := GeminiProfile(ProfileOptions{
		ProviderID:       "google-a",
		ClientID:         "client-id",
		AuthorizationURL: "https://login.example.com/authorize",
		TokenURL:         "https://login.example.com/token",
		Scopes:           []string{"custom"},
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
}

func TestProfileRequiresClientID(t *testing.T) {
	if _, err := OpenAIProfile(ProfileOptions{}); err == nil {
		t.Fatal("OpenAIProfile() error = nil, want client ID validation error")
	}
}
