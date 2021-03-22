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
		broker: NewBroker(ctx, ToolchainWatcher{
			Context: ctx,
			Client:  options.monClient,
		}),
		hashSrv: util.NewHashServer(),
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

// agent <-> scheduler
func (s *schedulerServer) StreamIncomingTasks(
	srv types.Scheduler_StreamIncomingTasksServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	s.broker.NewAgentTaskStream(srv)
	s.agentCount.Inc()
	defer s.agentCount.Dec()

	select {
	case <-srv.Context().Done():
	case <-s.srvContext.Done():
	}

	return nil
}

// consumerd <-> scheduler
func (s *schedulerServer) StreamOutgoingTasks(
	srv types.Scheduler_StreamOutgoingTasksServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	s.broker.NewConsumerdTaskStream(srv)

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
