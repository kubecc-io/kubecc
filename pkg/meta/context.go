package meta

import (
	"context"
)

type Context interface {
	context.Context
	ValueAccessors
	MetadataProviders() []Provider
}

func New(mps ...Provider) Context {
	providers := map[interface{}]Provider{}
	for _, mp := range mps {
		providers[mp.Key()] = mp
	}
	return &contextImpl{
		Context:   context.Background(),
		providers: providers,
	}
}
