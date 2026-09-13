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
