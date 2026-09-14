package credentials

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/phongsathornpt/kokekokkor/internal/domain/credential"
)

type memoryRepository struct {
	values map[credential.Ref][]byte
}

func (r *memoryRepository) Get(_ context.Context, ref credential.Ref) ([]byte, error) {
	value, ok := r.values[ref]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (r *memoryRepository) Put(_ context.Context, ref credential.Ref, value []byte) error {
	if r.values == nil {
		r.values = make(map[credential.Ref][]byte)
	}
	r.values[ref] = append([]byte(nil), value...)
	return nil
}

func (r *memoryRepository) Delete(_ context.Context, ref credential.Ref) error {
	delete(r.values, ref)
	return nil
}

func TestServiceSetAPIKeyPersistsAndApplies(t *testing.T) {
	repo := &memoryRepository{values: make(map[credential.Ref][]byte)}
	service := NewService(repo, map[string]string{"one": "old"})
	var applied map[string]string
	service.SetRuntimeApply(func(_ context.Context, values map[string]string) error {
		applied = cloneValues(values)
		return nil
	})

	if err := service.SetAPIKey(context.Background(), "one", "new"); err != nil {
		t.Fatalf("SetAPIKey() error = %v", err)
	}
	ref := credential.Ref{ProviderID: "one", Kind: credential.KindAPIKey}
	if got := string(repo.values[ref]); got != "new" {
		t.Fatalf("persisted value = %q", got)
	}
	if got := service.Snapshot()["one"]; got != "new" {
		t.Fatalf("runtime value = %q", got)
	}
	if applied["one"] != "new" {
		t.Fatalf("applied values = %#v", applied)
	}
}

func TestServiceRollsBackPersistenceWhenRuntimeApplyFails(t *testing.T) {
	ref := credential.Ref{ProviderID: "one", Kind: credential.KindAPIKey}
	repo := &memoryRepository{values: map[credential.Ref][]byte{ref: []byte("old")}}
	service := NewService(repo, map[string]string{"one": "old"})
	service.SetRuntimeApply(func(context.Context, map[string]string) error { return errors.New("apply failed") })

	if err := service.SetAPIKey(context.Background(), "one", "new"); err == nil {
		t.Fatal("SetAPIKey() error = nil")
	}
	if got := string(repo.values[ref]); got != "old" {
		t.Fatalf("persisted value after rollback = %q", got)
	}
	if got := service.Snapshot()["one"]; got != "old" {
		t.Fatalf("runtime value after rollback = %q", got)
	}
}

func TestServiceDeleteAPIKeyAppliesRemoval(t *testing.T) {
	ref := credential.Ref{ProviderID: "one", Kind: credential.KindAPIKey}
	repo := &memoryRepository{values: map[credential.Ref][]byte{ref: []byte("old")}}
	service := NewService(repo, map[string]string{"one": "old"})
	var applied map[string]string
	service.SetRuntimeApply(func(_ context.Context, values map[string]string) error {
		applied = cloneValues(values)
		return nil
	})

	if err := service.DeleteAPIKey(context.Background(), "one"); err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}
	if _, ok := repo.values[ref]; ok {
		t.Fatal("credential still persisted")
	}
	if service.HasAPIKey("one") {
		t.Fatal("HasAPIKey(one) = true")
	}
	if !reflect.DeepEqual(applied, map[string]string{}) {
		t.Fatalf("applied values = %#v", applied)
	}
}

func TestServiceReadOnlyRejectsMutation(t *testing.T) {
	service := NewService(nil, map[string]string{"one": "env"})
	if service.Editable() {
		t.Fatal("Editable() = true")
	}
	if err := service.SetAPIKey(context.Background(), "one", "new"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("SetAPIKey() error = %v", err)
	}
}

func TestServiceForgetProviderDropsInMemoryCredentialEvenWhenReadOnly(t *testing.T) {
	service := NewService(nil, map[string]string{"one": "env", "two": "keep"})
	service.ForgetProvider(" one ")
	got := service.Snapshot()
	if _, ok := got["one"]; ok {
		t.Fatalf("forgotten provider remains: %#v", got)
	}
	if got["two"] != "keep" {
		t.Fatalf("unrelated credential changed: %#v", got)
	}
}
