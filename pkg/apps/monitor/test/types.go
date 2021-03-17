package test

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"go.uber.org/atomic"
)

type TestStoreCreator struct {
	Count  *atomic.Int32
	Stores sync.Map // map[string]monitor.KeyValueStore
}

func (c *TestStoreCreator) NewStore(ctx context.Context) monitor.KeyValueStore {
	store := monitor.InMemoryStoreCreator.NewStore(ctx)
	c.Stores.Store(ctx, store)
	i := int32(0)
	c.Stores.Range(func(key, value interface{}) bool {
		i++
		return true
	})
	c.Count.Store(i)
	return store
}