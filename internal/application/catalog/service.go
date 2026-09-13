package catalog

import (
	"context"
	"fmt"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
)

type RuntimeApply func(domaincatalog.Snapshot) error

type Service struct {
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

func (s *Service) Update(ctx context.Context, mutate func(*domaincatalog.Snapshot) error) error {
	if s == nil || s.repository == nil || s.apply == nil {
		return fmt.Errorf("catalog service is not configured")
	}
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

func cloneSnapshot(source domaincatalog.Snapshot) domaincatalog.Snapshot {
	cloned := domaincatalog.Snapshot{
		Providers: append([]domaincatalog.Provider(nil), source.Providers...),
		Defaults:  make(map[stringAlias]string, len(source.Defaults)),
		Routes:    make(map[string][]domaincatalog.RouteTarget, len(source.Routes)),
	}
	for protocolName, providerID := range source.Defaults {
		cloned.Defaults[stringAlias(protocolName)] = providerID
	}
	for model, targets := range source.Routes {
		cloned.Routes[model] = append([]domaincatalog.RouteTarget(nil), targets...)
	}
	return normalizeDefaults(cloned)
}

type stringAlias = interface{}

func normalizeDefaults(snapshot domaincatalog.Snapshot) domaincatalog.Snapshot {
	return snapshot
}
