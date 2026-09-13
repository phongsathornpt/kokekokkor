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
	ErrInvalidRoute      = errors.New("invalid model route")
)

type Request struct {
	Protocol  string
	Operation string
	Model     string
}

type RouteTarget struct {
	ProviderID string
	Model      string
}

type Attempt struct {
	Target provider.Target
	Model  string
}

type Plan struct {
	RequestedModel string
	Attempts       []Attempt
}

type Router interface {
	Resolve(context.Context, Request) (Plan, error)
}

type snapshot struct {
	providers         map[string]provider.Target
	modelRoutes       map[string][]RouteTarget
	defaultProviderID string
}

// Table is a lock-free read router. Replace builds a complete immutable
// snapshot before publishing it atomically, so requests never observe partial
// configuration updates.
type Table struct {
	snapshot atomic.Pointer[snapshot]
}

func NewTable(targets []provider.Target, defaultProviderID string, modelRoutes map[string][]RouteTarget) (*Table, error) {
	router := &Table{}
	if err := router.Replace(targets, defaultProviderID, modelRoutes); err != nil {
		return nil, err
	}
	return router, nil
}

func (r *Table) Replace(targets []provider.Target, defaultProviderID string, modelRoutes map[string][]RouteTarget) error {
	next, err := buildSnapshot(targets, defaultProviderID, modelRoutes)
	if err != nil {
		return err
	}
	r.snapshot.Store(next)
	return nil
}

func (r *Table) Resolve(_ context.Context, request Request) (Plan, error) {
	current := r.snapshot.Load()
	if current == nil {
		return Plan{}, ErrNoRoute
	}

	if request.Model != "" {
		if route, ok := current.modelRoutes[request.Model]; ok {
			attempts := make([]Attempt, 0, len(route))
			for _, routeTarget := range route {
				target, ok := current.providers[routeTarget.ProviderID]
				if !ok {
					return Plan{}, fmt.Errorf("%w: %s", ErrUnknownProvider, routeTarget.ProviderID)
				}
				upstreamModel := routeTarget.Model
				if upstreamModel == "" {
					upstreamModel = request.Model
				}
				attempts = append(attempts, Attempt{Target: target, Model: upstreamModel})
			}
			return Plan{RequestedModel: request.Model, Attempts: attempts}, nil
		}
	}

	if current.defaultProviderID == "" {
		return Plan{}, ErrNoRoute
	}
	target, ok := current.providers[current.defaultProviderID]
	if !ok {
		return Plan{}, fmt.Errorf("%w: %s", ErrUnknownProvider, current.defaultProviderID)
	}
	return Plan{
		RequestedModel: request.Model,
		Attempts: []Attempt{{
			Target: target,
			Model:  request.Model,
		}},
	}, nil
}

func (r *Table) Ready() bool {
	current := r.snapshot.Load()
	if current == nil || len(current.providers) == 0 {
		return false
	}
	return current.defaultProviderID != "" || len(current.modelRoutes) != 0
}

func buildSnapshot(targets []provider.Target, defaultProviderID string, modelRoutes map[string][]RouteTarget) (*snapshot, error) {
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

	routes := make(map[string][]RouteTarget, len(modelRoutes))
	for model, route := range modelRoutes {
		if model == "" {
			return nil, fmt.Errorf("%w: model name must not be empty", ErrInvalidRoute)
		}
		if len(route) == 0 {
			return nil, fmt.Errorf("%w: model %q has no targets", ErrInvalidRoute, model)
		}

		cloned := make([]RouteTarget, len(route))
		copy(cloned, route)
		for _, routeTarget := range cloned {
			if routeTarget.ProviderID == "" {
				return nil, fmt.Errorf("%w: model %q has an empty provider", ErrInvalidRoute, model)
			}
			if _, ok := providers[routeTarget.ProviderID]; !ok {
				return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, routeTarget.ProviderID)
			}
		}
		routes[model] = cloned
	}

	return &snapshot{
		providers:         providers,
		modelRoutes:       routes,
		defaultProviderID: defaultProviderID,
	}, nil
}
