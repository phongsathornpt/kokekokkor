package oauth

import (
	"context"
	"errors"
	"testing"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

func TestMemoryStateRepositoryPrunesExpiredStatesOnPut(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	repo := NewMemoryStateRepository()
	repo.now = func() time.Time { return now }
	repo.items["expired"] = domainoauth.PendingAuthorization{
		State:     "expired",
		CreatedAt: now.Add(-2 * time.Minute),
		ExpiresAt: now.Add(-time.Minute),
	}

	if err := repo.Put(context.Background(), domainoauth.PendingAuthorization{
		State:     "current",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if _, ok := repo.items["expired"]; ok {
		t.Fatal("expired state was not pruned")
	}
	if _, ok := repo.items["current"]; !ok {
		t.Fatal("current state was not stored")
	}
}

func TestMemoryStateRepositoryEvictsOldestAtCapacity(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	repo := NewMemoryStateRepository()
	repo.now = func() time.Time { return now }
	repo.maxItems = 2
	for i, pending := range []domainoauth.PendingAuthorization{
		{State: "oldest", CreatedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(time.Minute)},
		{State: "newer", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)},
		{State: "newest", CreatedAt: now, ExpiresAt: now.Add(time.Minute)},
	} {
		if err := repo.Put(context.Background(), pending); err != nil {
			t.Fatalf("Put(%d) error = %v", i, err)
		}
	}
	if len(repo.items) != 2 {
		t.Fatalf("states = %d, want 2", len(repo.items))
	}
	if _, ok := repo.items["oldest"]; ok {
		t.Fatal("oldest state was not evicted")
	}
	for _, state := range []string{"newer", "newest"} {
		if _, ok := repo.items[state]; !ok {
			t.Fatalf("state %q missing after eviction", state)
		}
	}
}

func TestMemoryStateRepositoryExpiredStateStillConsumesAsExpired(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	repo := NewMemoryStateRepository()
	repo.now = func() time.Time { return now }
	if err := repo.Put(context.Background(), domainoauth.PendingAuthorization{
		State:     "state",
		CreatedAt: now.Add(-2 * time.Minute),
		ExpiresAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := repo.Consume(context.Background(), "state"); !errors.Is(err, ErrStateExpired) {
		t.Fatalf("Consume() error = %v, want ErrStateExpired", err)
	}
	if _, err := repo.Consume(context.Background(), "state"); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("second Consume() error = %v, want ErrStateNotFound", err)
	}
}
