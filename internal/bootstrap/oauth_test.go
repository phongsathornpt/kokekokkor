package bootstrap

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/config"
	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	sqlitestore "github.com/phongsathornpt/kokekokkor/internal/repository/sqlite"
)

func TestResolveOAuthRuntimeDisabled(t *testing.T) {
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")

	ctx := context.Background()
	runtime, err := resolveOAuthRuntime(ctx, config.Config{}, domaincatalog.Snapshot{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("resolveOAuthRuntime() error = %v", err)
	}
	if runtime.handler != nil || runtime.bearerTokens != nil {
		t.Fatalf("runtime = %#v, want empty", runtime)
	}
}

func TestResolveOAuthRuntimeCodexDefault(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", `{"v1":"`+key+`"}`)
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "default")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")

	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "oauth.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	snapshot := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{
			{ID: "openai", Protocol: provider.ProtocolOpenAI, BaseURL: "https://api.openai.com/v1", Enabled: true},
		},
		Defaults: map[provider.Protocol]string{
			provider.ProtocolOpenAI: "openai",
		},
	}
	if err := store.Replace(ctx, snapshot); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	cfg := config.Config{
		DefaultProviderID: "openai",
		Providers: []config.OpenAICompatible{
			{ID: "openai", BaseURL: "https://api.openai.com/v1"},
		},
	}

	runtime, err := resolveOAuthRuntime(ctx, cfg, snapshot, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("resolveOAuthRuntime() error = %v", err)
	}
	if runtime.handler == nil || runtime.bearerTokens == nil || runtime.tokens == nil {
		t.Fatalf("runtime = %#v, want non-nil fields", runtime)
	}
	if len(runtime.providerIDs) != 1 || runtime.providerIDs[0] != "openai" {
		t.Fatalf("providerIDs = %#v, want [\"openai\"]", runtime.providerIDs)
	}
}

func TestResolveOAuthRuntimeCodexProfilesJSON(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", `{"v1":"`+key+`"}`)
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", `[{"kind":"codex","provider_id":"codex-a"},{"kind":"chatgpt","provider_id":"chatgpt-b"}]`)

	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "oauth.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	snapshot := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{
			{ID: "codex-a", Protocol: provider.ProtocolOpenAI, BaseURL: "https://api.openai.com/v1", Enabled: true},
			{ID: "chatgpt-b", Protocol: provider.ProtocolOpenAI, BaseURL: "https://api.openai.com/v1", Enabled: true},
		},
	}
	if err := store.Replace(ctx, snapshot); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	cfg := config.Config{
		Providers: []config.OpenAICompatible{
			{ID: "codex-a", BaseURL: "https://api.openai.com/v1"},
			{ID: "chatgpt-b", BaseURL: "https://api.openai.com/v1"},
		},
	}

	runtime, err := resolveOAuthRuntime(ctx, cfg, snapshot, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("resolveOAuthRuntime() error = %v", err)
	}
	if len(runtime.providerIDs) != 2 || runtime.providerIDs[0] != "codex-a" || runtime.providerIDs[1] != "chatgpt-b" {
		t.Fatalf("providerIDs = %#v, want [\"codex-a\", \"chatgpt-b\"]", runtime.providerIDs)
	}
}

func TestResolveOAuthRuntimeCodexAllowsBuiltinPresetWithoutCatalogEntry(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", `{"v1":"`+key+`"}`)
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "default")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")

	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "oauth.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	// Snapshot has no providers configured.
	snapshot := domaincatalog.Snapshot{}

	runtime, err := resolveOAuthRuntime(ctx, config.Config{}, snapshot, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("resolveOAuthRuntime() error = %v", err)
	}
	if len(runtime.providerIDs) != 1 || runtime.providerIDs[0] != "openai" {
		t.Fatalf("providerIDs = %#v, want [\"openai\"]", runtime.providerIDs)
	}
}

func TestResolveOAuthRuntimeClaudeAndGitHub(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	t.Setenv("KOKEKOKKOR_CREDENTIAL_KEYS_JSON", `{"v1":"`+key+`"}`)
	t.Setenv("KOKEKOKKOR_CREDENTIAL_ACTIVE_KEY_VERSION", "v1")
	t.Setenv("KOKEKOKKOR_OAUTH_PUBLIC_BASE_URL", "https://gateway.example.com")
	t.Setenv("KOKEKOKKOR_OAUTH_GEMINI_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CODEX_CLIENT_ID", "")
	t.Setenv("KOKEKOKKOR_OAUTH_CLAUDE_CLIENT_ID", "default")
	t.Setenv("KOKEKOKKOR_OAUTH_GITHUB_CLIENT_ID", "default")
	t.Setenv("KOKEKOKKOR_OAUTH_PROFILES_JSON", "")

	ctx := context.Background()
	store, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "oauth.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	snapshot := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{
			{ID: "anthropic", Protocol: provider.ProtocolAnthropic, BaseURL: "https://api.anthropic.com", Enabled: true},
			{ID: "github", Protocol: provider.ProtocolOpenAI, BaseURL: "https://api.github.com", Enabled: true},
		},
	}

	cfg := config.Config{
		Anthropic: config.Anthropic{ID: "anthropic"},
	}

	runtime, err := resolveOAuthRuntime(ctx, cfg, snapshot, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("resolveOAuthRuntime() error = %v", err)
	}
	if len(runtime.providerIDs) != 2 || runtime.providerIDs[0] != "anthropic" || runtime.providerIDs[1] != "github" {
		t.Fatalf("providerIDs = %#v, want [\"anthropic\", \"github\"]", runtime.providerIDs)
	}
	if runtime.profiles["github"].FlowType != domainoauth.FlowTypeDeviceCode {
		t.Fatalf("github FlowType = %q, want device_code", runtime.profiles["github"].FlowType)
	}
}
