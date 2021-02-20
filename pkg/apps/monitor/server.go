package monitor

import (
	"bytes"
	"context"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/metrics/builtin"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/tinylib/msgp/msgp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type monitorServer struct {
	types.MonitorServer

	srvContext context.Context
	lg         *zap.SugaredLogger

	buckets       map[string]KeyValueStore
	bucketMutex   *sync.RWMutex
	listeners     map[string]map[string]types.Monitor_WatchServer
	listenerMutex *sync.RWMutex

	providers *builtin.Providers
}

func NewMonitorServer(ctx context.Context) types.MonitorServer {
	return &monitorServer{
		srvContext:    ctx,
		lg:            logkc.LogFromContext(ctx),
		buckets:       make(map[string]KeyValueStore),
		bucketMutex:   &sync.RWMutex{},
		listeners:     make(map[string]map[string]types.Monitor_WatchServer),
		listenerMutex: &sync.RWMutex{},
		providers:     &builtin.Providers{},
	}
}

func (m *monitorServer) encodeProviders() []byte {
	m.bucketMutex.RLock()
	defer m.bucketMutex.RUnlock()
	buf := new(bytes.Buffer)
	err := m.providers.EncodeMsg(msgp.NewWriter(buf))
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// bucketMutex must not be held by the same thread when calling this function
func (m *monitorServer) providersUpdated() {
	_, err := m.Post(m.srvContext, &types.Metric{
		Key: &types.Key{
			Bucket: builtin.Bucket,
			Name:   builtin.ProvidersKey,
		},
		Value: &types.Value{
			Data: m.encodeProviders(),
		},
	})
	if err != nil {
		panic(err)
	}
}

func (m *monitorServer) Connect(
	_ *types.Empty,
	srv types.Monitor_ConnectServer,
) error {
	id, err := types.IdentityFromContext(srv.Context())
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	m.bucketMutex.Lock()
	if _, ok := m.buckets[id.UUID]; ok {
		return status.Error(codes.AlreadyExists,
			"A client with the same identity is already connected")
	}
	store := NewInMemoryStore(srv.Context())
	m.buckets[id.UUID] = store
	m.providers.Items[id.UUID] = int32(id.Component)
	m.bucketMutex.Unlock()
	m.providersUpdated()

	<-srv.Context().Done()

	m.bucketMutex.Lock()
	delete(m.buckets, id.UUID)
	delete(m.providers.Items, id.UUID)
	m.bucketMutex.Unlock()
	m.providersUpdated()
	return nil
}

func (m *monitorServer) notify(metric *types.Metric) {
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

func (m *monitorServer) Post(ctx context.Context, metric *types.Metric) (*types.Empty, error) {
	m.bucketMutex.RLock()
	bucket := metric.Key.Bucket
	if store, ok := m.buckets[bucket]; ok {
		if store.CAS(metric.Key.Name, metric.Value.Data) {
			defer m.notify(metric)
		}
	} else {
		return nil, status.Error(codes.InvalidArgument, "No such bucket")
	}
	m.bucketMutex.RUnlock()
	return &types.Empty{}, nil
}

func (m *monitorServer) Watch(key *types.Key, srv types.Monitor_WatchServer) error {
	id, err := types.IdentityFromContext(srv.Context())
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	m.bucketMutex.RLock()

	var bucketCtx context.Context
	if bucket, ok := m.buckets[key.Bucket]; !ok {
		m.bucketMutex.RUnlock()
		return status.Error(codes.InvalidArgument, "No such bucket")
	} else {
		bucketCtx = bucket.Context()
	}

	m.listenerMutex.Lock()
	canonical := key.Canonical()
	if m.listeners[canonical] == nil {
		m.listeners[canonical] = make(map[string]types.Monitor_WatchServer)
	}
	m.listeners[canonical][id.UUID] = srv
	m.listenerMutex.Unlock()

	m.bucketMutex.RUnlock()

	select {
	case <-srv.Context().Done():
	case <-bucketCtx.Done():
	}

	m.listenerMutex.Lock()
	delete(m.listeners[canonical], id.UUID)
	m.listenerMutex.Unlock()
	return nil
}
