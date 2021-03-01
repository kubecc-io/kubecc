package monitor

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/meta"
	mmeta "github.com/cobalt77/kubecc/pkg/metrics/meta"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Receiver interface {
	Send(*types.Value) error
}

type MonitorServer struct {
	types.InternalMonitorServer
	types.ExternalMonitorServer

	srvContext context.Context
	lg         *zap.SugaredLogger

	buckets       map[string]KeyValueStore
	bucketMutex   *sync.RWMutex
	listeners     map[string]map[string]Receiver
	listenerMutex *sync.RWMutex

	storeCreator StoreCreator
	providers    *mmeta.Providers
}

func NewMonitorServer(
	ctx context.Context,
	storeCreator StoreCreator,
) *MonitorServer {
	srv := &MonitorServer{
		srvContext:    ctx,
		lg:            meta.Log(ctx),
		buckets:       make(map[string]KeyValueStore),
		bucketMutex:   &sync.RWMutex{},
		listeners:     make(map[string]map[string]Receiver),
		listenerMutex: &sync.RWMutex{},
		storeCreator:  storeCreator,
		providers:     &mmeta.Providers{},
	}
	srv.buckets[mmeta.Bucket] = storeCreator.NewStore(ctx)
	srv.providersUpdated()
	return srv
}

func (m *MonitorServer) encodeProviders() []byte {
	m.bucketMutex.RLock()
	defer m.bucketMutex.RUnlock()
	return tools.EncodeMsgp(m.providers)
}

// bucketMutex must not be held by the same thread when calling this function.
func (m *MonitorServer) providersUpdated() {
	err := m.post(&types.Metric{
		Key: &types.Key{
			Bucket: mmeta.Bucket,
			Name:   mmeta.Providers{}.Key(),
		},
		Value: &types.Value{
			Data: m.encodeProviders(),
		},
	})
	if err != nil {
		panic(err)
	}
}

func (m *MonitorServer) Stream(
	srv types.InternalMonitor_StreamServer,
) (streamError error) {
	if err := meta.CheckContext(srv.Context()); err != nil {
		return err
	}
	ctx := srv.Context()
	uuid := meta.UUID(ctx)
	component := meta.Component(ctx)

	m.bucketMutex.Lock()
	if _, ok := m.buckets[uuid]; ok {
		return status.Error(codes.AlreadyExists,
			"A client with the same identity is already connected")
	}
	bucketCtx, bucketCancel := context.WithCancel(context.Background())
	store := m.storeCreator.NewStore(bucketCtx)
	m.buckets[uuid] = store
	if m.providers.Items == nil {
		m.providers.Items = make(map[string]int32)
	}
	m.providers.Items[uuid] = int32(component)
	m.bucketMutex.Unlock()
	m.providersUpdated()

	m.lg.With(
		zap.String("component", component.Name()),
		types.ShortID(uuid),
	).Info(types.Monitor.Color().Add("Provider connected"))
	for {
		metric, err := srv.Recv()
		if err != nil {
			m.lg.Error(err)
			break
		}
		err = m.post(metric)
		if err != nil {
			m.lg.Error(err)
			streamError = err
			break
		}
	}
	m.lg.With(
		zap.String("component", component.Name()),
		types.ShortID(uuid),
	).Info(types.Monitor.Color().Add("Provider disconnected"))

	m.bucketMutex.Lock()
	bucketCancel()
	delete(m.buckets, uuid)
	delete(m.providers.Items, uuid)
	m.bucketMutex.Unlock()
	m.providersUpdated()
	return
}

func (m *MonitorServer) notify(metric *types.Metric) {
	m.listenerMutex.RLock()
	defer m.listenerMutex.RUnlock()

	if listeners, ok := m.listeners[metric.Key.Canonical()]; ok {
		for _, v := range listeners {
			err := v.Send(metric.Value)
			if err != nil {
				m.lg.With(zap.Error(err)).Error("Error notifying listener")
			}
		}
	}
}

func (m *MonitorServer) post(metric *types.Metric) error {
	m.bucketMutex.RLock()
	bucket := metric.Key.Bucket
	if store, ok := m.buckets[bucket]; ok {
		if store.CAS(metric.Key.Name, metric.Value.Data) {
			m.lg.With(
				zap.String("key", metric.Key.ShortID()),
			).Debug("Metric updated")
			defer m.notify(metric)
		}
	} else {
		m.bucketMutex.RUnlock()
		return status.Error(codes.InvalidArgument, "No such bucket")
	}
	m.bucketMutex.RUnlock()
	return nil
}

func (m *MonitorServer) Listen(
	key *types.Key,
	srv types.ExternalMonitor_ListenServer,
) error {
	if err := meta.CheckContext(srv.Context()); err != nil {
		return err
	}
	ctx := srv.Context()
	uuid := meta.UUID(ctx)
	m.lg.With(
		zap.String("component", meta.Component(ctx).Name()),
		types.ShortID(uuid),
	).Debug("Listener added")
	m.bucketMutex.RLock()

	var bucketCtx context.Context
	bucket, ok := m.buckets[key.Bucket]
	if !ok {
		m.bucketMutex.RUnlock()
		return status.Error(codes.InvalidArgument, "No such bucket")
	} else {
		bucketCtx = bucket.Context()
	}

	m.listenerMutex.Lock()
	canonical := key.Canonical()
	if m.listeners[canonical] == nil {
		m.listeners[canonical] = make(map[string]Receiver)
	}
	m.listeners[canonical][uuid] = srv
	m.listenerMutex.Unlock()

	// late join
	if value, ok := bucket.Get(key.Name); ok {
		err := srv.Send(&types.Value{
			Data: value,
		})
		if err != nil {
			m.lg.With(zap.Error(err)).Error("Error sending data to listener")
		}
	}

	m.bucketMutex.RUnlock()

	defer func() {
		m.listenerMutex.Lock()
		delete(m.listeners[canonical], uuid)
		m.listenerMutex.Unlock()
		m.lg.With(
			zap.String("component", meta.Component(ctx).Name()),
			types.ShortID(uuid),
		).Debug("Listener removed")
	}()

	select {
	case <-ctx.Done():
		return status.Error(codes.Canceled, "Context canceled")
	case <-bucketCtx.Done():
		return status.Error(codes.Aborted, "Bucket closed")
	}
}
