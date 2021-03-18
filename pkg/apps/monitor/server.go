package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/anypb"
)

type Receiver interface {
	Send(*anypb.Any) error
}

type MonitorServer struct {
	types.UnimplementedMonitorServer

	srvContext context.Context
	lg         *zap.SugaredLogger

	buckets       map[string]KeyValueStore
	listeners     map[string]map[string]Receiver
	providerMutex *sync.RWMutex
	listenerMutex *sync.RWMutex

	storeCreator StoreCreator
	providers    *metrics.Providers
}

func NewMonitorServer(
	ctx context.Context,
	storeCreator StoreCreator,
) *MonitorServer {
	srv := &MonitorServer{
		srvContext:    ctx,
		lg:            meta.Log(ctx),
		buckets:       make(map[string]KeyValueStore),
		listeners:     make(map[string]map[string]Receiver),
		providerMutex: &sync.RWMutex{},
		listenerMutex: &sync.RWMutex{},
		storeCreator:  storeCreator,
		providers:     &metrics.Providers{},
	}
	srv.buckets[metrics.MetaBucket] = storeCreator.NewStore(ctx)
	srv.providersUpdated()

	go srv.runPrometheusListener()
	return srv
}

func (m *MonitorServer) runPrometheusListener() {
	inMemoryListener := bufconn.Listen(1024 * 1024)
	inMemoryGrpcSrv := servers.NewServer(m.srvContext)
	types.RegisterMonitorServer(inMemoryGrpcSrv, m)

	go func() {
		if err := inMemoryGrpcSrv.Serve(inMemoryListener); err != nil {
			m.lg.With(
				zap.Error(err),
			).Error("Error serving internal metrics listener")
		}
	}()

	cc, err := servers.Dial(m.srvContext, meta.UUID(m.srvContext),
		servers.WithDialOpts(
			grpc.WithContextDialer(
				func(c context.Context, s string) (net.Conn, error) {
					return inMemoryListener.Dial()
				},
			),
			grpc.WithInsecure(),
		),
	)
	if err != nil {
		panic(err)
	}

	client := types.NewMonitorClient(cc)

	servePrometheusMetrics(m.srvContext, client)
}

// bucketMutex must not be held by the same thread when calling this function.
func (m *MonitorServer) providersUpdated() {
	any, err := anypb.New(m.providers)
	if err != nil {
		panic(err)
	}
	err = m.post(&types.Metric{
		Key: &types.Key{
			Bucket: metrics.MetaBucket,
			Name:   any.GetTypeUrl(),
		},
		Value: any,
	})
	if err != nil {
		panic(err)
	}
}

func providerIP(ctx context.Context) (string, error) {
	if p, ok := peer.FromContext(ctx); ok {
		switch addr := p.Addr.(type) {
		case *net.TCPAddr:
			return addr.IP.String(), nil
		default:
			return addr.String(), nil
		}
	}
	return "", status.Error(codes.InvalidArgument,
		"No peer information available")
}

func (m *MonitorServer) Stream(
	srv types.Monitor_StreamServer,
) (streamError error) {
	if err := meta.CheckContext(srv.Context()); err != nil {
		m.lg.With(
			zap.Error(err),
		).Error("Error handling provider stream")
		return err
	}
	ctx := srv.Context()
	addr, err := providerIP(srv.Context())
	if err != nil {
		m.lg.With(
			zap.Error(err),
		).Error("Error handling provider stream")
		return err
	}
	uuid := meta.UUID(ctx)
	component := meta.Component(ctx)

	m.providerMutex.Lock()
	if _, ok := m.buckets[uuid]; ok {
		return status.Error(codes.AlreadyExists,
			"A client with the same identity is already connected")
	}
	bucketCtx, bucketCancel := context.WithCancel(context.Background())
	store := m.storeCreator.NewStore(bucketCtx)
	m.buckets[uuid] = store
	if m.providers.Items == nil {
		m.providers.Items = make(map[string]*metrics.ProviderInfo)
	}
	m.providers.Items[uuid] = &metrics.ProviderInfo{
		UUID:      uuid,
		Component: component,
		Address:   addr,
	}
	providerCount.Inc()
	m.providerMutex.Unlock()
	m.providersUpdated()

	m.lg.With(
		zap.String("component", component.Name()),
		types.ShortID(uuid),
	).Info(types.Monitor.Color().Add("Provider connected"))
	for {
		metric, err := srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				m.lg.Debug(err)
			} else {
				m.lg.Error(err)
			}
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

	m.providerMutex.Lock()
	bucketCancel()
	delete(m.buckets, uuid)
	delete(m.providers.Items, uuid)
	providerCount.Dec()
	m.providerMutex.Unlock()
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

var storeContentsKey string

func init() {
	sc := &metrics.StoreContents{}
	any, err := anypb.New(sc)
	if err != nil {
		panic(err)
	}
	storeContentsKey = any.GetTypeUrl()
}

func (m *MonitorServer) notifyStoreMeta() {
	m.listenerMutex.RLock()
	defer m.listenerMutex.RUnlock()

	if listeners, ok := m.listeners[storeContentsKey]; ok {
		contents := &metrics.StoreContents{
			Buckets: []*metrics.BucketSpec{},
		}
		for k, v := range m.buckets {
			copied := map[string]*anypb.Any{}
			for _, key := range v.Keys() {
				if value, ok := v.Get(key); ok {
					any, err := anypb.New(value)
					if err != nil {
						m.lg.Error(err)
						continue
					}
					copied[key] = any
				}
			}
			contents.Buckets = append(contents.Buckets, &metrics.BucketSpec{
				Name: k,
				Data: copied,
			})
		}
		for _, v := range listeners {
			any, err := anypb.New(contents)
			if err != nil {
				panic(err)
			}
			err = v.Send(any)
			if err != nil {
				m.lg.With(zap.Error(err)).Error("Error sending data to listener")
			}
		}
	}
}

func (m *MonitorServer) post(metric *types.Metric) error {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()
	bucket := metric.Key.Bucket
	if store, ok := m.buckets[bucket]; ok {
		if metric.Value == nil {
			store.Delete(metric.Key.Name)
			return nil
		}
		contents, err := metric.Value.UnmarshalNew()
		if err != nil {
			return err
		}
		if store.CAS(metric.Key.Name, contents) {
			m.lg.With(
				zap.String("key", metric.Key.ShortID()),
			).Debug("Metric updated")
			metricsPostedTotal.Inc()
			defer func() {
				m.notify(metric)
				m.notifyStoreMeta()
			}()
		}
	} else {
		return status.Error(codes.InvalidArgument,
			fmt.Sprintf("No such bucket: '%s'", bucket))
	}
	return nil
}

func (m *MonitorServer) Listen(
	key *types.Key,
	srv types.Monitor_ListenServer,
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
	m.providerMutex.RLock()

	var bucketCtx context.Context
	bucket, ok := m.buckets[key.Bucket]
	if !ok {
		m.providerMutex.RUnlock()
		return status.Error(codes.FailedPrecondition,
			fmt.Sprintf("No such bucket: '%s'", key.Bucket))
	} else {
		bucketCtx = bucket.Context()
	}

	m.listenerMutex.Lock()
	canonical := key.Canonical()
	if m.listeners[canonical] == nil {
		m.listeners[canonical] = make(map[string]Receiver)
	}
	listenerCount.Inc()
	m.listeners[canonical][uuid] = srv
	m.listenerMutex.Unlock()

	// late join
	if value, ok := bucket.Get(key.Name); ok {
		any, err := anypb.New(value)
		if err != nil {
			panic(err)
		}
		err = srv.Send(any)
		if err != nil {
			m.lg.With(zap.Error(err)).Error("Error sending data to listener")
		}
	}
	m.notifyStoreMeta()

	m.providerMutex.RUnlock()

	defer func() {
		m.listenerMutex.Lock()
		delete(m.listeners[canonical], uuid)
		listenerCount.Dec()
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

func (m *MonitorServer) Whois(
	ctx context.Context,
	req *types.WhoisRequest,
) (*types.WhoisResponse, error) {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()

	if info, ok := m.providers.Items[req.GetUUID()]; ok {
		return &types.WhoisResponse{
			UUID:      req.GetUUID(),
			Address:   info.Address,
			Component: info.Component,
		}, nil
	}
	return nil, status.Error(codes.NotFound,
		"The requested provider was not found.")
}
