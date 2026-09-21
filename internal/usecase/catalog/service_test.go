package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domaincatalog "github.com/phongsathornpt/kokekokkor/internal/domain/catalog"
	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

type memoryRepository struct {
	snapshot    domaincatalog.Snapshot
	replaceErr  error
	replaceCall int
}

func (r *memoryRepository) Load(context.Context) (domaincatalog.Snapshot, error) {
	return cloneSnapshot(r.snapshot), nil
}

func (r *memoryRepository) Replace(_ context.Context, snapshot domaincatalog.Snapshot) error {
	r.replaceCall++
	if r.replaceErr != nil {
		return r.replaceErr
	}
	r.snapshot = cloneSnapshot(snapshot)
	return nil
}

func TestServiceUpdatesPersistenceAndRuntime(t *testing.T) {
	repo := &memoryRepository{snapshot: domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "one", Protocol: provider.ProtocolOpenAI, BaseURL: "https://one.example", Enabled: true}, {ID: "two", Protocol: provider.ProtocolOpenAI, BaseURL: "https://two.example", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolOpenAI: "one"},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}}
	var applied domaincatalog.Snapshot
	service := NewService(repo, func(snapshot domaincatalog.Snapshot) error {
		applied = cloneSnapshot(snapshot)
		return nil
	})
	if err := service.SetDefault(context.Background(), provider.ProtocolOpenAI, "two"); err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}
	if repo.snapshot.Defaults[provider.ProtocolOpenAI] != "two" || applied.Defaults[provider.ProtocolOpenAI] != "two" {
		t.Fatalf("persisted=%v applied=%v", repo.snapshot.Defaults, applied.Defaults)
	}
}

func TestServiceRollsBackPersistenceWhenRuntimeApplyFails(t *testing.T) {
	original := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "one", Protocol: provider.ProtocolOpenAI, BaseURL: "https://one.example", Enabled: true}, {ID: "two", Protocol: provider.ProtocolOpenAI, BaseURL: "https://two.example", Enabled: true}},
		Defaults:  map[provider.Protocol]string{provider.ProtocolOpenAI: "one"},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}
	repo := &memoryRepository{snapshot: cloneSnapshot(original)}
	service := NewService(repo, func(domaincatalog.Snapshot) error { return errors.New("apply failed") })
	if err := service.SetDefault(context.Background(), provider.ProtocolOpenAI, "two"); err == nil {
		t.Fatal("SetDefault() error = nil")
	}
	if !reflect.DeepEqual(repo.snapshot, original) {
		t.Fatalf("snapshot after rollback = %#v, want %#v", repo.snapshot, original)
	}
}

func TestServiceCreatesAndUpdatesProvider(t *testing.T) {
	repo := &memoryRepository{snapshot: domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{},
		Defaults:  map[provider.Protocol]string{},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}}
	service := NewService(repo, func(domaincatalog.Snapshot) error { return nil })

	if err := service.CreateProvider(context.Background(), domaincatalog.Provider{
		ID: " custom ", Protocol: provider.ProtocolOpenAI, BaseURL: " https://example.com/v1 ", Enabled: true,
	}); err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	if len(repo.snapshot.Providers) != 1 || repo.snapshot.Providers[0].ID != "custom" || repo.snapshot.Providers[0].BaseURL != "https://example.com/v1" {
		t.Fatalf("provider after create = %#v", repo.snapshot.Providers)
	}
	if err := service.CreateProvider(context.Background(), domaincatalog.Provider{
		ID: "custom", Protocol: provider.ProtocolOpenAI, BaseURL: "https://duplicate.example", Enabled: true,
	}); !errors.Is(err, ErrProviderExists) {
		t.Fatalf("duplicate CreateProvider() error = %v", err)
	}

	if err := service.UpdateProvider(context.Background(), "custom", domaincatalog.Provider{
		Protocol: provider.ProtocolAnthropic, BaseURL: " https://api.anthropic.com ", Enabled: false,
	}); err != nil {
		t.Fatalf("UpdateProvider() error = %v", err)
	}
	got := repo.snapshot.Providers[0]
	if got.ID != "custom" || got.Protocol != provider.ProtocolAnthropic || got.BaseURL != "https://api.anthropic.com" || got.Enabled {
		t.Fatalf("provider after update = %#v", got)
	}
}

func TestServiceDeleteProviderRejectsReferences(t *testing.T) {
	base := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{
			{ID: "one", Protocol: provider.ProtocolOpenAI, BaseURL: "https://one.example", Enabled: true},
			{ID: "two", Protocol: provider.ProtocolOpenAI, BaseURL: "https://two.example", Enabled: true},
		},
		Defaults: map[provider.Protocol]string{provider.ProtocolOpenAI: "one"},
		Routes:   map[string][]domaincatalog.RouteTarget{},
	}
	repo := &memoryRepository{snapshot: cloneSnapshot(base)}
	service := NewService(repo, func(domaincatalog.Snapshot) error { return nil })
	if err := service.DeleteProvider(context.Background(), "one"); !errors.Is(err, ErrProviderInUse) {
		t.Fatalf("DeleteProvider(default) error = %v", err)
	}

	repo.snapshot.Defaults = map[provider.Protocol]string{}
	repo.snapshot.Routes = map[string][]domaincatalog.RouteTarget{"portable": {{ProviderID: "one"}}}
	if err := service.DeleteProvider(context.Background(), "one"); !errors.Is(err, ErrProviderInUse) {
		t.Fatalf("DeleteProvider(route) error = %v", err)
	}
}

func TestServiceDeleteProviderAppliesBeforeDestructivePersistence(t *testing.T) {
	original := domaincatalog.Snapshot{
		Providers: []domaincatalog.Provider{{ID: "one", Protocol: provider.ProtocolOpenAI, BaseURL: "https://one.example", Enabled: true}},
		Defaults:  map[provider.Protocol]string{},
		Routes:    map[string][]domaincatalog.RouteTarget{},
	}
	repo := &memoryRepository{snapshot: cloneSnapshot(original), replaceErr: errors.New("disk failed")}
	var applied []domaincatalog.Snapshot
	service := NewService(repo, func(snapshot domaincatalog.Snapshot) error {
		applied = append(applied, cloneSnapshot(snapshot))
		return nil
	})

	if err := service.DeleteProvider(context.Background(), "one"); err == nil {
		t.Fatal("DeleteProvider() error = nil")
	}
	if repo.replaceCall != 1 {
		t.Fatalf("Replace() calls = %d", repo.replaceCall)
	}
	if len(applied) != 2 {
		t.Fatalf("runtime apply sequence = %#v", applied)
	}
	if len(applied[0].Providers) != 0 || !reflect.DeepEqual(applied[1], original) {
		t.Fatalf("runtime rollback sequence = %#v", applied)
	}
	if !reflect.DeepEqual(repo.snapshot, original) {
		t.Fatalf("persisted snapshot changed = %#v", repo.snapshot)
	}
}
