/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Receiver interface {
	Send(*anypb.Any) error
}

type MonitorServer struct {
	types.UnimplementedMonitorServer
	metrics.StatusController

	srvContext context.Context
	lg         *zap.SugaredLogger
	uuid       string

	buckets map[string]KeyValueStore

	// todo: refactor this
	// map[key]map[listener uuid]map[arbitrary id]Receiver
	listeners     map[string]map[string]map[string]Receiver
	providerMutex *sync.RWMutex
	listenerMutex *sync.RWMutex
	metricsTotal  *atomic.Int64
	storeCreator  StoreCreator
	providers     *metrics.Providers
}

func NewMonitorServer(
	ctx context.Context,
	conf config.MonitorSpec,
	storeCreator StoreCreator,
) *MonitorServer {
	uuid := meta.UUID(ctx)
	srv := &MonitorServer{
		srvContext: ctx,
		lg:         meta.Log(ctx),
		uuid:       uuid,
		buckets: map[string]KeyValueStore{
			uuid: storeCreator.NewStore(ctx),
		},
		listeners:     make(map[string]map[string]map[string]Receiver),
		providerMutex: &sync.RWMutex{},
		listenerMutex: &sync.RWMutex{},
		storeCreator:  storeCreator,
		metricsTotal:  atomic.NewInt64(0),
		providers: &metrics.Providers{
			Items: map[string]*metrics.ProviderInfo{
				uuid: {
					UUID:      uuid,
					Component: types.Monitor,
					Address:   "0.0.0.0",
				},
			},
		},
	}
	srv.BeginInitialize()
	defer srv.EndInitialize()

	srv.providerMutex.Lock()
	providerCount.Inc()
	srv.buckets[clients.MetaBucket] = storeCreator.NewStore(ctx)
	srv.providersUpdated()
	srv.providerMutex.Unlock()

	if conf.ServePrometheusMetrics {
		go srv.runPrometheusListener()
	}

	srv.startMetricsProvider()

	return srv
}

func (m *MonitorServer) postInternal(any *anypb.Any) {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()
	err := m.post(&types.Metric{
		Key: &types.Key{
			Bucket: m.uuid,
			Name:   any.TypeUrl,
		},
		Value: any,
	})
	if err != nil {
		m.lg.Error(err)
	}
}

func (m *MonitorServer) postTotals() {
	postedTotal := &metrics.MetricsPostedTotal{
		Total: m.metricsTotal.Load(),
	}
	any, err := anypb.New(postedTotal)
	if err != nil {
		panic(err)
	}
	m.postInternal(any)
}

func (m *MonitorServer) postListeners() {
	m.listenerMutex.RLock()
	total := 0
	for _, m := range m.listeners {
		for _, v := range m {
			total += len(v)
		}
	}
	m.listenerMutex.RUnlock()
	any, err := anypb.New(&metrics.ListenerCount{
		Count: int32(total),
	})
	if err != nil {
		panic(err)
	}
	m.postInternal(any)
}

func (m *MonitorServer) postProviders() {
	m.providerMutex.RLock()
	total := len(m.providers.GetItems())
	m.providerMutex.RUnlock()
	any, err := anypb.New(&metrics.ProviderCount{
		Count: int32(total),
	})
	if err != nil {
		panic(err)
	}
	m.postInternal(any)
}

func (m *MonitorServer) postHealthUpdates() {
	c := m.StreamHealthUpdates()
	for {
		health := <-c
		any, err := anypb.New(health)
		if err != nil {
			panic(err)
		}
		m.postInternal(any)
	}
}

func (m *MonitorServer) startMetricsProvider() {
	go m.postHealthUpdates()
	util.RunPeriodic(m.srvContext, 5*time.Second, 0.5, false,
		m.postTotals, m.postListeners, m.postProviders)
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

func (m *MonitorServer) incMetricsPostedTotal() {
	m.metricsTotal.Inc()
	metricsPostedTotal.Inc()
}

// providerMutex must be write-locked when calling this function.
func (m *MonitorServer) providersUpdated() {
	any, err := anypb.New(proto.Clone(m.providers))
	if err != nil {
		panic(err)
	}
	// the providerMutex lock requirement is satisfied by the requirements of
	// calling providersUpdated
	err = m.post(&types.Metric{
		Key: &types.Key{
			Bucket: clients.MetaBucket,
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
	m.providers.Items[uuid] = &metrics.ProviderInfo{
		UUID:      uuid,
		Component: component,
		Address:   addr,
	}
	providerCount.Inc()
	m.providersUpdated()
	m.providerMutex.Unlock()

	m.lg.With(
		zap.String("component", component.Name()),
		types.ShortID(uuid),
	).Info(types.Monitor.Color().Add("Provider connected"))
	for {
		metric, err := srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
				m.lg.Debug(err)
			} else {
				m.lg.Error(err)
			}
			break
		}

		m.providerMutex.RLock()
		err = m.post(metric)
		m.providerMutex.RUnlock()

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
	delete(m.buckets, uuid)
	delete(m.providers.Items, uuid)
	providerCount.Dec()
	// Important: the providerMutex must stay write-locked when canceling the
	// bucket context, otherwise listeners will be removed and may not be
	// notified of this cancellation.
	bucketCancel()
	m.providersUpdated()
	m.providerMutex.Unlock()
	return
}

func (m *MonitorServer) notify(metric *types.Metric) {
	m.listenerMutex.RLock()
	defer m.listenerMutex.RUnlock()

	if listeners, ok := m.listeners[metric.Key.Canonical()]; ok {
		for _, receivers := range listeners {
			for _, receiver := range receivers {
				err := receiver.Send(metric.Value)
				if err != nil && status.Code(err) != codes.Canceled {
					m.lg.With(zap.Error(err)).Error("Error notifying listener")
				}
			}
		}
	}
}

// providerMutex must be locked (read or write) when calling this function.
func (m *MonitorServer) post(metric *types.Metric) error {
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
			m.incMetricsPostedTotal()
			m.notify(metric)
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
	listenerID := meta.UUID(ctx)
	nonce := uuid.NewString()
	m.lg.With(
		zap.String("component", meta.Component(ctx).Name()),
		"id", types.FormatShortID(listenerID, 6, types.ElideCenter),
		"nonce", types.FormatShortID(nonce, 4, types.ElideCenter),
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
		m.listeners[canonical] = make(map[string]map[string]Receiver)
	}
	if m.listeners[canonical][listenerID] == nil {
		m.listeners[canonical][listenerID] = make(map[string]Receiver)
	}
	listenerCount.Inc()
	m.listeners[canonical][listenerID][nonce] = srv
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

	m.providerMutex.RUnlock()

	defer func() {
		// Locking the provider mutex here ensures listeners stay alive while
		// a provider is being deleted, so that they can be properly notified
		// that a provider has been deleted by canceling the associated bucket
		// context.
		m.providerMutex.Lock()
		// Mutex lock order is important here!
		m.listenerMutex.Lock()

		delete(m.listeners[canonical][listenerID], nonce)
		listenerCount.Dec()
		m.listenerMutex.Unlock()
		m.providerMutex.Unlock()

		m.lg.With(
			zap.String("component", meta.Component(ctx).Name()),
			"id", types.FormatShortID(listenerID, 6, types.ElideCenter),
			"nonce", types.FormatShortID(nonce, 4, types.ElideCenter),
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

func (m *MonitorServer) GetMetric(_ context.Context, key *types.Key) (*types.Metric, error) {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()

	if bucket, ok := m.buckets[key.Bucket]; ok {
		msg, exists := bucket.Get(key.Name)
		if !exists {
			return nil, status.Error(codes.NotFound, "No such key")
		}
		any, err := anypb.New(msg)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return &types.Metric{
			Key:   key,
			Value: any,
		}, nil
	}

	return nil, status.Error(codes.NotFound, "No such bucket")
}

func (m *MonitorServer) GetBuckets(
	ctx context.Context,
	_ *types.Empty,
) (*types.BucketList, error) {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()

	list := &types.BucketList{
		Buckets: make([]*types.Bucket, 0, len(m.buckets)),
	}
	for k := range m.buckets {
		list.Buckets = append(list.Buckets, &types.Bucket{
			Name: k,
		})
	}

	return list, nil
}
func (m *MonitorServer) GetKeys(
	ctx context.Context,
	bucket *types.Bucket,
) (*types.KeyList, error) {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()

	if b, ok := m.buckets[bucket.GetName()]; ok {
		keys := b.Keys()
		list := &types.KeyList{
			Keys: make([]*types.Key, len(keys)),
		}
		for i, k := range keys {
			list.Keys[i] = &types.Key{
				Bucket: bucket.GetName(),
				Name:   k,
			}
		}
		return list, nil
	}

	return nil, status.Error(codes.NotFound, "No such bucket")
}
