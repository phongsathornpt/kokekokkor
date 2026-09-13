package credentials

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
)

var ErrReadOnly = errors.New("credential store is read only")

type RuntimeApply func(context.Context, map[string]string) error

type Service struct {
	mu         sync.RWMutex
	repository Repository
	values     map[string]string
	apply      RuntimeApply
}

func NewService(repository Repository, initial map[string]string) *Service {
	return &Service{repository: repository, values: cloneValues(initial)}
}

func (s *Service) SetRuntimeApply(apply RuntimeApply) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apply = apply
}

func (s *Service) Editable() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repository != nil && s.apply != nil
}

func (s *Service) Snapshot() map[string]string {
	if s == nil {
		return map[string]string{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneValues(s.values)
}

func (s *Service) HasAPIKey(providerID string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[strings.TrimSpace(providerID)] != ""
}

func (s *Service) SetAPIKey(ctx context.Context, providerID, value string) error {
	providerID = strings.TrimSpace(providerID)
	value = strings.TrimSpace(value)
	if providerID == "" {
		return fmt.Errorf("credential provider ID must not be empty")
	}
	if value == "" {
		return fmt.Errorf("API key must not be empty")
	}
	return s.update(ctx, providerID, &value)
}

func (s *Service) DeleteAPIKey(ctx context.Context, providerID string) error {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return fmt.Errorf("credential provider ID must not be empty")
	}
	return s.update(ctx, providerID, nil)
}

func (s *Service) update(ctx context.Context, providerID string, value *string) error {
	if s == nil {
		return ErrReadOnly
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repository == nil {
		return ErrReadOnly
	}
	if s.apply == nil {
		return fmt.Errorf("credential runtime apply is not configured")
	}

	current := cloneValues(s.values)
	next := cloneValues(current)
	oldValue, hadOld := current[providerID]
	ref := credential.Ref{ProviderID: providerID, Kind: credential.KindAPIKey}

	var err error
	if value == nil {
		delete(next, providerID)
		err = s.repository.Delete(ctx, ref)
	} else {
		next[providerID] = *value
		err = s.repository.Put(ctx, ref, []byte(*value))
	}
	if err != nil {
		return err
	}
	if err := s.apply(ctx, cloneValues(next)); err != nil {
		rollbackErr := s.rollback(ctx, ref, hadOld, oldValue)
		if rollbackErr != nil {
			return fmt.Errorf("apply runtime credentials: %w; rollback persistence: %v", err, rollbackErr)
		}
		return fmt.Errorf("apply runtime credentials: %w", err)
	}
	s.values = next
	return nil
}

func (s *Service) rollback(ctx context.Context, ref credential.Ref, hadOld bool, oldValue string) error {
	if hadOld {
		return s.repository.Put(ctx, ref, []byte(oldValue))
	}
	return s.repository.Delete(ctx, ref)
}

func cloneValues(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for providerID, value := range source {
		cloned[providerID] = value
	}
	return cloned
}
