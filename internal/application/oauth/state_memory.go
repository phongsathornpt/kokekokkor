package oauth

import (
	"context"
	"sync"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

const maxPendingOAuthStates = 1024

type MemoryStateRepository struct {
	mu       sync.Mutex
	items    map[string]domainoauth.PendingAuthorization
	now      func() time.Time
	maxItems int
}

func NewMemoryStateRepository() *MemoryStateRepository {
	return &MemoryStateRepository{
		items:    make(map[string]domainoauth.PendingAuthorization),
		now:      time.Now,
		maxItems: maxPendingOAuthStates,
	}
}

func (r *MemoryStateRepository) Put(_ context.Context, pending domainoauth.PendingAuthorization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now().UTC()
	r.pruneExpiredLocked(now)
	if _, exists := r.items[pending.State]; !exists && r.maxItems > 0 && len(r.items) >= r.maxItems {
		r.evictOldestLocked()
	}
	r.items[pending.State] = pending
	return nil
}

func (r *MemoryStateRepository) Consume(_ context.Context, state string) (domainoauth.PendingAuthorization, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending, ok := r.items[state]
	if !ok {
		return domainoauth.PendingAuthorization{}, ErrStateNotFound
	}
	delete(r.items, state)
	if !pending.ExpiresAt.IsZero() && !r.now().Before(pending.ExpiresAt) {
		return domainoauth.PendingAuthorization{}, ErrStateExpired
	}
	return pending, nil
}

func (r *MemoryStateRepository) pruneExpiredLocked(now time.Time) {
	for state, pending := range r.items {
		if !pending.ExpiresAt.IsZero() && !now.Before(pending.ExpiresAt) {
			delete(r.items, state)
		}
	}
}

func (r *MemoryStateRepository) evictOldestLocked() {
	var oldestState string
	var oldestCreated time.Time
	for state, pending := range r.items {
		if oldestState == "" || pending.CreatedAt.Before(oldestCreated) || pending.CreatedAt.Equal(oldestCreated) && state < oldestState {
			oldestState = state
			oldestCreated = pending.CreatedAt
		}
	}
	if oldestState != "" {
		delete(r.items, oldestState)
	}
}
