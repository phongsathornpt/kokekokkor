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
	snapshot domaincatalog.Snapshot
}

func (r *memoryRepository) Load(context.Context) (domaincatalog.Snapshot, error) {
	return cloneSnapshot(r.snapshot), nil
}
func (r *memoryRepository) Replace(_ context.Context, snapshot domaincatalog.Snapshot) error {
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
