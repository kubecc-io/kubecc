package scheduler

import (
	"context"
	"errors"
	"io"
	"time"

	scmetrics "github.com/cobalt77/kubecc/pkg/apps/scheduler/metrics"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/common"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type schedulerServer struct {
	types.UnimplementedSchedulerServer

	monClient   types.InternalMonitorClient
	cacheClient types.CacheClient

	srvContext      context.Context
	lg              *zap.SugaredLogger
	scheduler       *Scheduler
	metricsProvider metrics.Provider
	hashSrv         *util.HashServer

	agentCount     *atomic.Int32
	consumerdCount *atomic.Int32
}

type SchedulerServerOptions struct {
	schedulerOptions []schedulerOption
	monClient        types.InternalMonitorClient
	cacheClient      types.CacheClient
}

type schedulerServerOption func(*SchedulerServerOptions)

func (o *SchedulerServerOptions) Apply(opts ...schedulerServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithSchedulerOptions(opts ...schedulerOption) schedulerServerOption {
	return func(o *SchedulerServerOptions) {
		o.schedulerOptions = opts
	}
}

func WithMonitorClient(monClient types.InternalMonitorClient) schedulerServerOption {
	return func(o *SchedulerServerOptions) {
		o.monClient = monClient
	}
}

func WithCacheClient(cacheClient types.CacheClient) schedulerServerOption {
	return func(o *SchedulerServerOptions) {
		o.cacheClient = cacheClient
	}
}

func NewSchedulerServer(
	ctx context.Context,
	opts ...schedulerServerOption,
) *schedulerServer {
	options := SchedulerServerOptions{}
	options.Apply(opts...)

	srv := &schedulerServer{
		srvContext:     ctx,
		lg:             meta.Log(ctx),
		monClient:      options.monClient,
		cacheClient:    options.cacheClient,
		scheduler:      NewScheduler(ctx, options.schedulerOptions...),
		agentCount:     atomic.NewInt32(0),
		consumerdCount: atomic.NewInt32(0),
		hashSrv:        util.NewHashServer(),
	}

	if options.monClient != nil {
		srv.metricsProvider = metrics.NewMonitorProvider(ctx, options.monClient)
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
				ExpirationTime: time.Now().Add(1 * time.Hour).UnixNano(),
				CpuSecondsUsed: resp.GetCpuSecondsUsed(),
			},
		},
	})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		s.lg.With(
			zap.Error(err),
		).Error("Error sending data to the cache server")
	}
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	if err := meta.CheckContext(ctx); err != nil {
		return nil, err
	}
	span, sctx, err := servers.StartSpanFromServer(ctx, "schedule-compile")
	if err != nil {
		s.lg.Error(err)
	} else {
		ctx = sctx
		defer span.Finish()
	}
	peer, ok := peer.FromContext(ctx)
	if ok {
		s.lg.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	cacheMiss := false
	var reqHash string
	if s.cacheClient != nil {
		reqHash = s.hashSrv.Hash(req)
		obj, err := s.cacheClient.Pull(ctx, &types.PullRequest{
			Key: &types.CacheKey{
				Hash: reqHash,
			},
		})
		switch status.Code(err) {
		case codes.OK:
			s.lg.Info("Cache Hit")
			return &types.CompileResponse{
				CompileResult: types.CompileResponse_Success,
				Data: &types.CompileResponse_CompiledSource{
					CompiledSource: obj.GetData(),
				},
			}, nil
		case codes.NotFound:
			cacheMiss = true
		default:
			s.lg.With(
				zap.Error(err),
			).Error("Error querying cache server")
		}
	}
	resp, err := s.scheduler.Schedule(ctx, req)
	if err == nil &&
		resp.CompileResult == types.CompileResponse_Success &&
		cacheMiss {
		go s.cacheTransaction(reqHash, resp)
	}
	return resp, err
}

func (s *schedulerServer) handleClientConnection(srv grpc.ServerStream) error {
	done := make(chan error)
	ctx := srv.Context()
	go func() {
		defer close(done)
		for {
			metadata := &types.Metadata{}
			err := srv.RecvMsg(metadata)
			if err != nil {
				if errors.Is(err, io.EOF) {
					s.lg.Debug(err)
					done <- nil
				} else {
					s.lg.Error(err)
					done <- err
				}
				return
			}
			if err := s.scheduler.SetToolchains(
				ctx, metadata.Toolchains.GetItems()); err != nil {
				s.lg.Error(err)
			}
		}
	}()
	return <-done
}

func (s *schedulerServer) ConnectAgent(
	srv types.Scheduler_ConnectAgentServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}
	if err := s.scheduler.AgentConnected(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	s.metricsProvider.Post(&scmetrics.AgentCount{
		Count: s.agentCount.Inc(),
	})
	defer func() {
		s.metricsProvider.Post(&scmetrics.AgentCount{
			Count: s.agentCount.Dec(),
		})
	}()

	return s.handleClientConnection(srv)
}

func (s *schedulerServer) ConnectConsumerd(
	srv types.Scheduler_ConnectConsumerdServer,
) error {
	ctx := srv.Context()
	if err := meta.CheckContext(ctx); err != nil {
		s.lg.Error(err)
		return err
	}
	if err := s.scheduler.ConsumerdConnected(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	s.metricsProvider.Post(&scmetrics.CdCount{
		Count: s.consumerdCount.Inc(),
	})
	defer func() {
		s.metricsProvider.Post(&scmetrics.CdCount{
			Count: s.consumerdCount.Dec(),
		})
	}()

	return s.handleClientConnection(srv)
}

func (s *schedulerServer) postAlive() {
	s.metricsProvider.Post(&common.Alive{})
}

func (s *schedulerServer) postCounts() {
	s.metricsProvider.Post(&scmetrics.AgentCount{
		Count: s.agentCount.Load(),
	})
	s.metricsProvider.Post(&scmetrics.CdCount{
		Count: s.consumerdCount.Load(),
	})
}

func (s *schedulerServer) postTotals() {
	stats := s.scheduler.TaskStats()
	s.metricsProvider.Post(stats.completedTotal)
	s.metricsProvider.Post(stats.failedTotal)
	s.metricsProvider.Post(stats.requestsTotal)
}

func (s *schedulerServer) postAgentStats() {
	for _, stat := range <-s.scheduler.CalcAgentStats() {
		s.metricsProvider.Post(stat.agentTasksTotal, stat.agentCtx)
		s.metricsProvider.Post(stat.agentWeight, stat.agentCtx)
	}
}

func (s *schedulerServer) postConsumerdStats() {
	for _, stat := range <-s.scheduler.CalcConsumerdStats() {
		s.metricsProvider.Post(stat.cdRemoteTasksTotal, stat.consumerdCtx)
	}
}

func (s *schedulerServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.postAlive()
	s.postCounts()

	slowTimer := util.NewJitteredTimer(5*time.Second, 0.5) // 5-7.5 sec
	go func() {
		for {
			<-slowTimer
			s.postTotals()
			s.postAgentStats()
			s.postConsumerdStats()
		}
	}()
}
