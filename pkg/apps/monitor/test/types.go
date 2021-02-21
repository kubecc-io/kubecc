package test

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
)

//go:generate msgp

type TestStoreCreator struct {
	Count  *atomic.Int32
	Stores sync.Map // map[string]monitor.KeyValueStore
}

func (c *TestStoreCreator) NewStore(ctx context.Context) monitor.KeyValueStore {
	id, ok := types.IdentityFromContext(ctx)
	if !ok {
		idIncoming, err := types.IdentityFromIncomingContext(ctx)
		if err != nil {
			panic(err)
		}
		id = idIncoming
	}
	store := monitor.InMemoryStoreCreator.NewStore(ctx)
	c.Stores.Store(id.UUID, store)
	c.Count.Inc()
	return store
}

type TestKey1 struct {
	Counter int `msg:"counter"`
}

func (k TestKey1) Key() string {
	return "TestKey1"
}

type TestKey2 struct {
	Value string `msg:"value"`
}

func (k TestKey2) Key() string {
	return "TestKey2"
}
