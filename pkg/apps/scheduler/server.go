package scheduler

import (
	"context"
	"time"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type schedulerServer struct {
	types.UnimplementedSchedulerServer

	monClient   types.MonitorClient
	cacheClient types.CacheClient

	srvContext      context.Context
	lg              *zap.SugaredLogger
	metricsProvider metrics.Provider
	hashSrv         *util.HashServer
	broker          *Broker

	agentCount     *atomic.Int64
	consumerdCount *atomic.Int64
}

type SchedulerServerOptions struct {
	monClient   types.MonitorClient
	cacheClient types.CacheClient
}

type SchedulerServerOption func(*SchedulerServerOptions)

func (o *SchedulerServerOptions) Apply(opts ...SchedulerServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithMonitorClient(monClient types.MonitorClient) SchedulerServerOption {
	return func(o *SchedulerServerOptions) {
		o.monClient = monClient
	}
}

func WithCacheClient(cacheClient types.CacheClient) SchedulerServerOption {
	return func(o *SchedulerServerOptions) {
		o.cacheClient = cacheClient
	}
}

func NewSchedulerServer(
	ctx context.Context,
	opts ...SchedulerServerOption,
) *schedulerServer {
	options := SchedulerServerOptions{}
	options.Apply(opts...)

	srv := &schedulerServer{
		srvContext:     ctx,
		lg:             meta.Log(ctx),
		monClient:      options.monClient,
		cacheClient:    options.cacheClient,
		agentCount:     atomic.NewInt64(0),
		consumerdCount: atomic.NewInt64(0),
		broker:         NewBroker(ctx, options.monClient),
		hashSrv:        util.NewHashServer(),
	}

	if options.monClient != nil {
		srv.metricsProvider = clients.NewMonitorProvider(
			ctx, options.monClient, clients.Discard)
	} else {
		srv.metricsProvider = metrics.NewNoopProvider()
	}
	return srv
}

func (s *schedulerServer) cacheTransaction(
	requestHash string,
	resp *types.CompileResponse,
) {
	_, err := s.cacheClient.Push(s.srvContext, &types.PushRequest{
		Key: &types.CacheKey{
			Hash: requestHash,
		},
		Object: &types.CacheObject{
			Data: resp.GetCompiledSource(),
			Metadata: &types.CacheObjectMeta{
				ExpirationDate: time.Now().Add(1 * time.Hour).UnixNano(),
			},
		},
	})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		s.lg.With(
			zap.Error(err),
		).Error("Error sending data to the cache server")
	}
}

// func (s *schedulerServer) Compile(
// 	ctx context.Context,
// 	req *types.CompileRequest,
// ) (*types.CompileResponse, error) {
// 	if err := meta.CheckContext(ctx); err != nil {
// 		return nil, err
// 	}
// 	span, sctx, err := servers.StartSpanFromServer(ctx, "schedule-compile")
// 	if err != nil {
// 		s.lg.Error(err)
// 	} else {
// 		ctx = sctx
// 		defer span.Finish()
// 	}
// 	peer, ok := peer.FromContext(ctx)
// 	if ok {
// 		s.lg.With("peer", peer.Addr.String()).Info("Schedule requested")
// 	}
// 	cacheMiss := false
// 	var reqHash string
// 	if s.cacheClient != nil {
// 		reqHash = s.hashSrv.Hash(req)
// 		obj, err := s.cacheClient.Pull(ctx, &types.PullRequest{
// 			Key: &types.CacheKey{
// 				Hash: reqHash,
// 			},
// 		})
// 		switch status.Code(err) {
// 		case codes.OK:
// 			s.lg.Info("Cache Hit")
// 			return &types.CompileResponse{
// 				CompileResult: types.CompileResponse_Success,
// 				Data: &types.CompileResponse_CompiledSource{
// 					CompiledSource: obj.GetData(),
// 				},
// 			}, nil
// 		case codes.NotFound:
// 			cacheMiss = true
// 		default:
// 			s.lg.With(
// 				zap.Error(err),
// 			).Error("Error querying cache server")
// 		}
// 	}

// 	resp, err := s.scheduler.Schedule(ctx, req)
// 	if err == nil &&
// 		resp.CompileResult == types.CompileResponse_Success &&
// 		cacheMiss {
// 		go s.cacheTransaction(reqHash, resp)
// 	}
// 	return resp, err
// }

// func (s *schedulerServer) handleClientConnection(srv grpc.ServerStream) error {
// 	done := make(chan error)
// 	ctx := srv.Context()
// 	go func() {
// 		defer close(done)
// 		for {
// 			metadata := &types.Metadata{}
// 			err := srv.RecvMsg(metadata)
// 			if err != nil {
// 				if errors.Is(err, io.EOF) {
// 					s.lg.Debug(err)
// 					done <- nil
// 				} else {
// 					s.lg.Error(err)
// 					done <- err
// 				}
// 				return
// 			}
// 			if err := s.scheduler.SetToolchains(
// 				ctx, metadata.Toolchains.GetItems()); err != nil {
// 				s.lg.Error(err)
// 			}
// 		}
// 	}()
// 	return <-done
// }

// func (s *schedulerServer) ConnectAgent(
// 	srv types.Scheduler_ConnectAgentServer,
// ) error {
// 	ctx := srv.Context()
// 	if err := meta.CheckContext(ctx); err != nil {
// 		s.lg.Error(err)
// 		return err
// 	}
// 	if err := s.scheduler.AgentConnected(ctx); err != nil {
// 		s.lg.Error(err)
// 		return err
// 	}

// 	s.metricsProvider.Post(&scmetrics.AgentCount{
// 		Count: s.agentCount.Inc(),
// 	})
// 	defer func() {
// 		s.metricsProvider.Post(&scmetrics.AgentCount{
// 			Count: s.agentCount.Dec(),
// 		})
// 	}()

// 	return s.handleClientConnection(srv)
// }

// func (s *schedulerServer) StreamMetadata(
// 	srv types.Scheduler_StreamMetadataServer,
// ) error {
// 	ctx := srv.Context()
// 	if err := meta.CheckContext(ctx); err != nil {
// 		s.lg.Error(err)
// 		return err
// 	}

// 	if err := s.scheduler.ConsumerdConnected(ctx); err != nil {
// 		s.lg.Error(err)
// 		return err
// 	}

// 	s.metricsProvider.Post(&scmetrics.CdCount{
// 		Count: s.consumerdCount.Inc(),
// 	})
// 	defer func() {
// 		s.metricsProvider.Post(&scmetrics.CdCount{
// 			Count: s.consumerdCount.Dec(),
// 		})
// 	}()

// 	return s.handleClientConnection(srv)
// }

func (s *schedulerServer) StreamIncomingTasks(
	srv types.Scheduler_StreamIncomingTasksServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	s.broker.HandleAgentTaskStream(srv)
	s.agentCount.Inc()
	defer s.agentCount.Dec()

	select {
	case <-srv.Context().Done():
	case <-s.srvContext.Done():
	}

	return nil
}

func (s *schedulerServer) StreamOutgoingTasks(
	srv types.Scheduler_StreamOutgoingTasksServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	s.broker.HandleConsumerdTaskStream(srv)

	s.metricsProvider.Post(&metrics.ConsumerdCount{
		Count: s.consumerdCount.Inc(),
	})
	defer func() {
		s.metricsProvider.Post(&metrics.ConsumerdCount{
			Count: s.consumerdCount.Dec(),
		})
	}()

	select {
	case <-srv.Context().Done():
	case <-s.srvContext.Done():
	}

	return nil
}

func (s *schedulerServer) postCounts() {
	s.metricsProvider.Post(&metrics.AgentCount{
		Count: s.agentCount.Load(),
	})
	s.metricsProvider.Post(&metrics.ConsumerdCount{
		Count: s.consumerdCount.Load(),
	})
}

func (s *schedulerServer) postTotals() {
	stats := s.broker.TaskStats()
	s.metricsProvider.Post(stats.completedTotal)
	s.metricsProvider.Post(stats.failedTotal)
	s.metricsProvider.Post(stats.requestsTotal)
}

func (s *schedulerServer) postAgentStats() {
	for _, stat := range <-s.broker.CalcAgentStats() {
		s.metricsProvider.PostContext(stat.agentTasksTotal, stat.agentCtx)
	}
}

func (s *schedulerServer) postConsumerdStats() {
	for _, stat := range <-s.broker.CalcConsumerdStats() {
		s.metricsProvider.PostContext(stat.cdRemoteTasksTotal, stat.consumerdCtx)
	}
}

func (s *schedulerServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")

	slowTimer := util.NewJitteredTimer(5*time.Second, 0.5) // 5-7.5 sec
	go func() {
		for {
			<-slowTimer
			s.postCounts()
			s.postTotals()
			s.postAgentStats()
			s.postConsumerdStats()
		}
	}()
}
