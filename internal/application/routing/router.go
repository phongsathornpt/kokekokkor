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
	ErrInvalidProtocol   = errors.New("invalid provider protocol")
)

type Request struct {
	Protocol  provider.Protocol
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
	providers          map[string]provider.Target
	modelRoutes        map[string][]RouteTarget
	defaultProviderIDs map[provider.Protocol]string
}

// Table is a lock-free read router. Replace builds a complete immutable
// snapshot before publishing it atomically, so requests never observe partial
// configuration updates.
type Table struct {
	snapshot atomic.Pointer[snapshot]
}

// NewTable preserves the original OpenAI-compatible default-provider API.
func NewTable(targets []provider.Target, defaultProviderID string, modelRoutes map[string][]RouteTarget) (*Table, error) {
	defaults := make(map[provider.Protocol]string, 1)
	if defaultProviderID != "" {
		defaults[provider.ProtocolOpenAI] = defaultProviderID
	}
	return NewProtocolTable(targets, defaults, modelRoutes)
}

func NewProtocolTable(targets []provider.Target, defaultProviderIDs map[provider.Protocol]string, modelRoutes map[string][]RouteTarget) (*Table, error) {
	router := &Table{}
	if err := router.ReplaceProtocols(targets, defaultProviderIDs, modelRoutes); err != nil {
		return nil, err
	}
	return router, nil
}

// Replace preserves the original OpenAI-compatible default-provider API.
func (r *Table) Replace(targets []provider.Target, defaultProviderID string, modelRoutes map[string][]RouteTarget) error {
	defaults := make(map[provider.Protocol]string, 1)
	if defaultProviderID != "" {
		defaults[provider.ProtocolOpenAI] = defaultProviderID
	}
	return r.ReplaceProtocols(targets, defaults, modelRoutes)
}

func (r *Table) ReplaceProtocols(targets []provider.Target, defaultProviderIDs map[provider.Protocol]string, modelRoutes map[string][]RouteTarget) error {
	next, err := buildSnapshot(targets, defaultProviderIDs, modelRoutes)
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

	protocol := request.Protocol
	if protocol == "" {
		protocol = provider.ProtocolOpenAI
	}
	defaultProviderID := current.defaultProviderIDs[protocol]
	if defaultProviderID == "" {
		return Plan{}, ErrNoRoute
	}
	target, ok := current.providers[defaultProviderID]
	if !ok {
		return Plan{}, fmt.Errorf("%w: %s", ErrUnknownProvider, defaultProviderID)
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
	return len(current.defaultProviderIDs) != 0 || len(current.modelRoutes) != 0
}

func buildSnapshot(targets []provider.Target, defaultProviderIDs map[provider.Protocol]string, modelRoutes map[string][]RouteTarget) (*snapshot, error) {
	providers := make(map[string]provider.Target, len(targets))
	for _, target := range targets {
		if target.ID == "" {
			return nil, ErrInvalidProviderID
		}
		if _, exists := providers[target.ID]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateProvider, target.ID)
		}
		target.Protocol = target.EffectiveProtocol()
		switch target.Protocol {
		case provider.ProtocolOpenAI, provider.ProtocolAnthropic:
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidProtocol, target.Protocol)
		}
		providers[target.ID] = target
	}

	defaults := make(map[provider.Protocol]string, len(defaultProviderIDs))
	for protocol, providerID := range defaultProviderIDs {
		if providerID == "" {
			continue
		}
		switch protocol {
		case provider.ProtocolOpenAI, provider.ProtocolAnthropic:
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidProtocol, protocol)
		}
		target, ok := providers[providerID]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, providerID)
		}
		if target.Protocol != protocol {
			return nil, fmt.Errorf("%w: default %s provider %q speaks %s", ErrInvalidProtocol, protocol, providerID, target.Protocol)
		}
		defaults[protocol] = providerID
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
		providers:          providers,
		modelRoutes:        routes,
		defaultProviderIDs: defaults,
	}, nil
}
