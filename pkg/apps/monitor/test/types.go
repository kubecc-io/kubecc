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

const (
	TestKey1Name = "test-key1"
	TestKey2Name = "test-key2"
)

var (
	TestKey1Type *TestKey1
	TestKey2Type *TestKey2
)

type TestKey1 struct {
	Counter int `msg:"counter"`
}

type TestKey2 struct {
	Value string `msg:"value"`
}
