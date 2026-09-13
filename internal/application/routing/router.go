package routing

import (
	"context"
	"errors"

	"github.com/phongsathornpt/kokekokkor/internal/domain/provider"
)

var ErrNoRoute = errors.New("no upstream route configured")

type Request struct {
	Protocol  string
	Operation string
	Model     string
}

type Router interface {
	Resolve(context.Context, Request) (provider.Target, error)
}

type Static struct {
	target *provider.Target
}

func NewStatic(target *provider.Target) *Static {
	return &Static{target: target}
}

func (r *Static) Resolve(_ context.Context, _ Request) (provider.Target, error) {
	if r.target == nil {
		return provider.Target{}, ErrNoRoute
	}
	return *r.target, nil
}

func (r *Static) Ready() bool {
	return r.target != nil
}
