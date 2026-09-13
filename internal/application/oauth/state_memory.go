package oauth

import (
	"context"
	"sync"
	"time"

	domainoauth "github.com/phongsathornpt/kokekokkor/internal/domain/oauth"
)

type MemoryStateRepository struct {
	mu    sync.Mutex
	items map[string]domainoauth.PendingAuthorization
	now   func() time.Time
}

func NewMemoryStateRepository() *MemoryStateRepository {
	return &MemoryStateRepository{
		items: make(map[string]domainoauth.PendingAuthorization),
		now:   time.Now,
	}
}

func (r *MemoryStateRepository) Put(_ context.Context, pending domainoauth.PendingAuthorization) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
