package routing

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

var (
	ErrNoRoute           = errors.New("no upstream route configured")
	ErrInvalidProviderID = errors.New("provider ID must not be empty")
	ErrDuplicateProvider = errors.New("duplicate provider ID")
	ErrUnknownProvider   = errors.New("route references unknown provider")
)

type Request struct {
	Protocol  string
	Operation string
	Model     string
}

type Router interface {
	Resolve(context.Context, Request) (provider.Target, error)
}

type snapshot struct {
	providers         map[string]provider.Target
	modelRoutes       map[string]string
	defaultProviderID string
}

// Table is a lock-free read router. Replace builds a complete immutable
// snapshot before publishing it atomically, so requests never observe partial
// configuration updates.
type Table struct {
	snapshot atomic.Pointer[snapshot]
}

func NewTable(targets []provider.Target, defaultProviderID string, modelRoutes map[string]string) (*Table, error) {
	router := &Table{}
	if err := router.Replace(targets, defaultProviderID, modelRoutes); err != nil {
		return nil, err
	}
	return router, nil
}

func (r *Table) Replace(targets []provider.Target, defaultProviderID string, modelRoutes map[string]string) error {
	next, err := buildSnapshot(targets, defaultProviderID, modelRoutes)
	if err != nil {
		return err
	}
	r.snapshot.Store(next)
	return nil
}

func (r *Table) Resolve(_ context.Context, request Request) (provider.Target, error) {
	current := r.snapshot.Load()
	if current == nil {
		return provider.Target{}, ErrNoRoute
	}

	providerID := current.defaultProviderID
	if request.Model != "" {
		if routedProviderID, ok := current.modelRoutes[request.Model]; ok {
			providerID = routedProviderID
		}
	}
	if providerID == "" {
		return provider.Target{}, ErrNoRoute
	}

	target, ok := current.providers[providerID]
	if !ok {
		return provider.Target{}, fmt.Errorf("%w: %s", ErrUnknownProvider, providerID)
	}
	return target, nil
}

func (r *Table) Ready() bool {
	current := r.snapshot.Load()
	if current == nil || len(current.providers) == 0 {
		return false
	}
	return current.defaultProviderID != "" || len(current.modelRoutes) != 0
}

func buildSnapshot(targets []provider.Target, defaultProviderID string, modelRoutes map[string]string) (*snapshot, error) {
	providers := make(map[string]provider.Target, len(targets))
	for _, target := range targets {
		if target.ID == "" {
			return nil, ErrInvalidProviderID
		}
		if _, exists := providers[target.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateProvider, target.ID)
		}
		providers[target.ID] = target
	}

	if defaultProviderID != "" {
		if _, ok := providers[defaultProviderID]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, defaultProviderID)
		}
	}

	routes := make(map[string]string, len(modelRoutes))
	for model, providerID := range modelRoutes {
		if model == "" {
			return nil, fmt.Errorf("model route name must not be empty")
		}
		if _, ok := providers[providerID]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, providerID)
		}
		routes[model] = providerID
	}

	return &snapshot{
		providers:         providers,
		modelRoutes:       routes,
		defaultProviderID: defaultProviderID,
	}, nil
}
