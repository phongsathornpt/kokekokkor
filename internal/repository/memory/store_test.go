package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	domaincredential "github.com/phongsathornpt/kokekokkor/internal/domain/credential"
	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
	"github.com/phongsathornpt/kokekokkor/internal/domain/responsestate"
	"github.com/phongsathornpt/kokekokkor/internal/repository/memory"
	appcredential "github.com/phongsathornpt/kokekokkor/internal/usecase/credential"
	appoauth "github.com/phongsathornpt/kokekokkor/internal/usecase/oauth"
)

func TestCatalogRepository(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	defer store.Close()

	initial, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(initial.Providers) != 0 {
		t.Fatalf("expected 0 providers, got %d", len(initial.Providers))
	}

	snapshot := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{
			{ID: "openai-main", Protocol: provider.ProtocolOpenAI, BaseURL: "https://api.openai.com", Enabled: true},
		},
		Defaults: map[provider.Protocol]string{
			provider.ProtocolOpenAI: "openai-main",
		},
		Routes: map[string][]domaincatalog.RouteTarget{
			"gpt-4o": {{ProviderID: "openai-main", Model: "gpt-4o"}},
		},
	}

	if err := store.Replace(ctx, snapshot); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load() after replace error = %v", err)
	}
	if len(loaded.Providers) != 1 || loaded.Providers[0].ID != "openai-main" {
		t.Fatalf("loaded providers mismatch: %+v", loaded.Providers)
	}
	if loaded.Defaults[provider.ProtocolOpenAI] != "openai-main" {
		t.Fatalf("loaded defaults mismatch: %+v", loaded.Defaults)
	}
}

func TestCredentialRepository(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	defer store.Close()

	ref := domaincredential.Ref{ProviderID: "test-provider", Kind: domaincredential.KindAPIKey}

	_, err := store.Get(ctx, ref)
	if !errors.Is(err, appcredential.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := store.Put(ctx, ref, []byte("sk-test-key")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	val, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(val) != "sk-test-key" {
		t.Fatalf("expected sk-test-key, got %s", string(val))
	}

	if err := store.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get(ctx, ref)
	if !errors.Is(err, appcredential.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after Delete, got %v", err)
	}
}

func TestTokenStore(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	tokens := store.Tokens()

	_, err := tokens.Get(ctx, "chatgpt")
	if !errors.Is(err, appoauth.ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}

	tokenSet := domainoauth.TokenSet{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}

	if err := tokens.Put(ctx, "chatgpt", tokenSet); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := tokens.Get(ctx, "chatgpt")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.AccessToken != "access-123" {
		t.Fatalf("expected access-123, got %s", got.AccessToken)
	}

	if err := tokens.Delete(ctx, "chatgpt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = tokens.Get(ctx, "chatgpt")
	if !errors.Is(err, appoauth.ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound after Delete, got %v", err)
	}
}

func TestResponseStateAndConversation(t *testing.T) {
	ctx := context.Background()
	store := memory.New()

	// Response state
	rec := responsestate.Record{
		Messages:    []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hello"}}}},
		Continuable: true,
		Payload:     []byte(`{"status":"in_progress"}`),
		Status:      "in_progress",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	if err := store.SaveResponse(ctx, "resp-1", rec); err != nil {
		t.Fatalf("SaveResponse() error = %v", err)
	}

	gotRec, err := store.LoadResponse(ctx, "resp-1")
	if err != nil {
		t.Fatalf("LoadResponse() error = %v", err)
	}
	if gotRec.Status != "in_progress" || !gotRec.Continuable {
		t.Fatalf("unexpected record: %+v", gotRec)
	}

	// Expired response
	expiredRec := responsestate.Record{
		Status:    "expired",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	_ = store.SaveResponse(ctx, "resp-expired", expiredRec)
	_, err = store.LoadResponse(ctx, "resp-expired")
	if !errors.Is(err, responsestate.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for expired response, got %v", err)
	}

	// Delete response
	if err := store.DeleteResponse(ctx, "resp-1"); err != nil {
		t.Fatalf("DeleteResponse() error = %v", err)
	}
	_, err = store.LoadResponse(ctx, "resp-1")
	if !errors.Is(err, responsestate.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Conversation
	conv := responsestate.Conversation{
		ID:          "conv-1",
		CreatedAt:   time.Now(),
		Metadata:    map[string]string{"user": "alice"},
		Messages:    []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock{Text: "hi"}}}},
		Continuable: true,
	}

	if err := store.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation() error = %v", err)
	}

	gotConv, err := store.LoadConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("LoadConversation() error = %v", err)
	}
	if gotConv.Metadata["user"] != "alice" {
		t.Fatalf("expected alice metadata, got %s", gotConv.Metadata["user"])
	}

	if err := store.DeleteConversation(ctx, "conv-1"); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}
	_, err = store.LoadConversation(ctx, "conv-1")
	if !errors.Is(err, responsestate.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete conversation, got %v", err)
	}
}

func TestConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	defer store.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			ref := domaincredential.Ref{ProviderID: "prov", Kind: domaincredential.KindAPIKey}
			_ = store.Put(ctx, ref, []byte("key"))
			_, _ = store.Get(ctx, ref)
		}(i)
		go func(idx int) {
			defer wg.Done()
			_ = store.SaveResponse(ctx, "resp", responsestate.Record{Status: "ok"})
			_, _ = store.LoadResponse(ctx, "resp")
		}(i)
	}
	wg.Wait()
}
