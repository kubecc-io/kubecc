package test

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"go.uber.org/atomic"
)

type TestStoreCreator struct {
	Count  *atomic.Int32
	Stores sync.Map // map[string]monitor.KeyValueStore
}

func (c *TestStoreCreator) NewStore(ctx context.Context) monitor.KeyValueStore {
	store := monitor.InMemoryStoreCreator.NewStore(ctx)
	c.Stores.Store(ctx.Value(mdkeys.UUIDKey), store)
	c.Count.Inc()
	return store
}
