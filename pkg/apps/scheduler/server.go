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
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.UnimplementedSchedulerServer

	monClient       types.InternalMonitorClient
	srvContext      context.Context
	lg              *zap.SugaredLogger
	scheduler       *Scheduler
	metricsProvider metrics.Provider

	agentCount     *atomic.Int32
	consumerdCount *atomic.Int32
}

type SchedulerServerOptions struct {
	schedulerOptions []schedulerOption
	monClient        types.InternalMonitorClient
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
		scheduler:      NewScheduler(ctx, options.schedulerOptions...),
		agentCount:     atomic.NewInt32(0),
		consumerdCount: atomic.NewInt32(0),
	}

	if options.monClient != nil {
		srv.metricsProvider = metrics.NewMonitorProvider(ctx, options.monClient)
	} else {
		srv.metricsProvider = metrics.NewNoopProvider()
	}
	return srv
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
	return s.scheduler.Schedule(ctx, req)
}

func (s *schedulerServer) ConnectAgent(
	srv types.Scheduler_ConnectAgentServer,
) error {
	if err := meta.CheckContext(srv.Context()); err != nil {
		s.lg.Error(err)
		return err
	}
	lg := s.lg
	ctx := srv.Context()
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

	go func() {
		for {
			metadata, err := srv.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					lg.Debug(err)
				} else {
					lg.Error(err)
				}
				return
			}
			if err := s.scheduler.SetToolchains(
				ctx, metadata.Toolchains.GetItems()); err != nil {
				lg.Error(err)
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func (s *schedulerServer) ConnectConsumerd(
	srv types.Scheduler_ConnectConsumerdServer,
) error {
	lg := s.lg
	ctx := srv.Context()

	if err := s.scheduler.ConsumerdConnected(ctx); err != nil {
		s.lg.Error(err)
		return err
	}

	lg.Info(types.Scheduler.Color().Add("Consumerd connected"))
	defer lg.Info(types.Scheduler.Color().Add("Consumerd disconnected"))

	s.metricsProvider.Post(&scmetrics.CdCount{
		Count: s.consumerdCount.Inc(),
	})
	defer func() {
		s.metricsProvider.Post(&scmetrics.CdCount{
			Count: s.consumerdCount.Dec(),
		})
	}()

	go func() {
		for {
			metadata, err := srv.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					lg.Debug(err)
				} else {
					lg.Error(err)
				}
				return
			}
			if err := s.scheduler.SetToolchains(
				ctx, metadata.Toolchains.GetItems()); err != nil {
				lg.Error(err)
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func (s *schedulerServer) postAlive() {
	s.metricsProvider.Post(&common.Alive{})
}
func (s *schedulerServer) postTotals() {
	stats := s.scheduler.TaskStats()
	s.metricsProvider.Post(stats.completedTotal)
	s.metricsProvider.Post(stats.failedTotal)
	s.metricsProvider.Post(stats.requestsTotal)
}

func (s *schedulerServer) postAgentStats() {
	for _, stat := range <-s.scheduler.CalcAgentStats() {
		s.metricsProvider.Post(stat.agentTasksTotal)
		s.metricsProvider.Post(stat.agentWeight)
	}
}

func (s *schedulerServer) postConsumerdStats() {
	for _, stat := range <-s.scheduler.CalcConsumerdStats() {
		s.metricsProvider.Post(stat.cdRemoteTasksTotal)
	}
}

func (s *schedulerServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.postAlive()

	slowTimer := util.NewJitteredTimer(1*time.Second, 1.0)
	go func() {
		for {
			<-slowTimer
			s.postTotals()
			s.postAgentStats()
			s.postConsumerdStats()
		}
	}()
}
