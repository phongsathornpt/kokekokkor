package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

var (
	ErrProviderNotFound = errors.New("catalog provider not found")
	ErrProviderExists   = errors.New("catalog provider already exists")
	ErrProviderInUse    = errors.New("catalog provider is still referenced")
)

type RuntimeApply func(domaincatalog.Snapshot) error

type Service struct {
	mu         sync.Mutex
	repository Repository
	apply      RuntimeApply
}

func NewService(repository Repository, apply RuntimeApply) *Service {
	return &Service{repository: repository, apply: apply}
}

func (s *Service) Load(ctx context.Context) (domaincatalog.Snapshot, error) {
	if s == nil || s.repository == nil {
		return domaincatalog.Snapshot{}, fmt.Errorf("catalog repository is not configured")
	}
	return s.repository.Load(ctx)
}

func (s *Service) CreateProvider(ctx context.Context, item domaincatalog.Provider) error {
	item.ID = strings.TrimSpace(item.ID)
	item.BaseURL = strings.TrimSpace(item.BaseURL)
	return s.Update(ctx, func(snapshot *domaincatalog.Snapshot) error {
		for _, configured := range snapshot.Providers {
			if configured.ID == item.ID {
				return fmt.Errorf("%w: %s", ErrProviderExists, item.ID)
			}
		}
		snapshot.Providers = append(snapshot.Providers, item)
		return nil
	})
}

func (s *Service) UpdateProvider(ctx context.Context, providerID string, item domaincatalog.Provider) error {
	providerID = strings.TrimSpace(providerID)
	item.ID = providerID
	item.BaseURL = strings.TrimSpace(item.BaseURL)
	return s.Update(ctx, func(snapshot *domaincatalog.Snapshot) error {
		for i := range snapshot.Providers {
			if snapshot.Providers[i].ID == providerID {
				snapshot.Providers[i] = item
				return nil
			}
		}
		return fmt.Errorf("%w: %s", ErrProviderNotFound, providerID)
	})
}

func (s *Service) DeleteProvider(ctx context.Context, providerID string) error {
	providerID = strings.TrimSpace(providerID)
	return s.update(ctx, true, func(snapshot *domaincatalog.Snapshot) error {
		index := -1
		for i, configured := range snapshot.Providers {
			if configured.ID == providerID {
				index = i
				break
			}
		}
		if index < 0 {
			return fmt.Errorf("%w: %s", ErrProviderNotFound, providerID)
		}
		for protocolName, defaultProviderID := range snapshot.Defaults {
			if defaultProviderID == providerID {
				return fmt.Errorf("%w: provider %q is the %s default", ErrProviderInUse, providerID, protocolName)
			}
		}
		for model, targets := range snapshot.Routes {
			for _, target := range targets {
				if target.ProviderID == providerID {
					return fmt.Errorf("%w: provider %q is used by route %q", ErrProviderInUse, providerID, model)
				}
			}
		}
		snapshot.Providers = append(snapshot.Providers[:index], snapshot.Providers[index+1:]...)
		return nil
	})
}

func (s *Service) SetProviderEnabled(ctx context.Context, providerID string, enabled bool) error {
	providerID = strings.TrimSpace(providerID)
	return s.Update(ctx, func(snapshot *domaincatalog.Snapshot) error {
		for i := range snapshot.Providers {
			if snapshot.Providers[i].ID == providerID {
				snapshot.Providers[i].Enabled = enabled
				return nil
			}
		}
		return fmt.Errorf("%w: %s", ErrProviderNotFound, providerID)
	})
}

func (s *Service) SetDefault(ctx context.Context, protocolName provider.Protocol, providerID string) error {
	providerID = strings.TrimSpace(providerID)
	return s.Update(ctx, func(snapshot *domaincatalog.Snapshot) error {
		if providerID == "" {
			delete(snapshot.Defaults, protocolName)
			return nil
		}
		snapshot.Defaults[protocolName] = providerID
		return nil
	})
}

func (s *Service) SetRoute(ctx context.Context, model string, targets []domaincatalog.RouteTarget) error {
	model = strings.TrimSpace(model)
	return s.Update(ctx, func(snapshot *domaincatalog.Snapshot) error {
		snapshot.Routes[model] = append([]domaincatalog.RouteTarget(nil), targets...)
		return nil
	})
}

func (s *Service) DeleteRoute(ctx context.Context, model string) error {
	model = strings.TrimSpace(model)
	return s.Update(ctx, func(snapshot *domaincatalog.Snapshot) error {
		delete(snapshot.Routes, model)
		return nil
	})
}

func (s *Service) Update(ctx context.Context, mutate func(*domaincatalog.Snapshot) error) error {
	return s.update(ctx, false, mutate)
}

func (s *Service) update(ctx context.Context, runtimeFirst bool, mutate func(*domaincatalog.Snapshot) error) error {
	if s == nil || s.repository == nil || s.apply == nil {
		return fmt.Errorf("catalog service is not configured")
	}
	if mutate == nil {
		return fmt.Errorf("catalog mutation is not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.repository.Load(ctx)
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}
	next := cloneSnapshot(current)
	if err := mutate(&next); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}
	if runtimeFirst {
		return s.applyThenPersist(ctx, current, next)
	}
	if err := s.repository.Replace(ctx, next); err != nil {
		return fmt.Errorf("persist catalog: %w", err)
	}
	if err := s.apply(next); err != nil {
		if rollbackErr := s.repository.Replace(ctx, current); rollbackErr != nil {
			return fmt.Errorf("apply runtime catalog: %w; rollback persistence: %v", err, rollbackErr)
		}
		return fmt.Errorf("apply runtime catalog: %w", err)
	}
	return nil
}

func (s *Service) applyThenPersist(ctx context.Context, current, next domaincatalog.Snapshot) error {
	if err := s.apply(next); err != nil {
		return fmt.Errorf("apply runtime catalog: %w", err)
	}
	if err := s.repository.Replace(ctx, next); err != nil {
		if rollbackErr := s.apply(current); rollbackErr != nil {
			return fmt.Errorf("persist catalog: %w; rollback runtime: %v", err, rollbackErr)
		}
		return fmt.Errorf("persist catalog: %w", err)
	}
	return nil
}

func cloneSnapshot(source domaincatalog.Snapshot) domaincatalog.Snapshot {
	cloned := domaincatalog.Snapshot{
		Providers: append([]domaincatalog.Provider(nil), source.Providers...),
		Defaults:  make(map[provider.Protocol]string, len(source.Defaults)),
		Routes:    make(map[string][]domaincatalog.RouteTarget, len(source.Routes)),
	}
	for protocolName, providerID := range source.Defaults {
		cloned.Defaults[protocolName] = providerID
	}
	for model, targets := range source.Routes {
		cloned.Routes[model] = append([]domaincatalog.RouteTarget(nil), targets...)
	}
	return cloned
}
