package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.SchedulerServer

	srvContext context.Context
	lg         *zap.SugaredLogger
	scheduler  *Scheduler
}

func NewSchedulerServer(
	ctx context.Context,
	opts ...schedulerOption,
) *schedulerServer {
	srv := &schedulerServer{
		srvContext: ctx,
		lg:         logkc.LogFromContext(ctx),
		scheduler:  NewScheduler(ctx, opts...),
	}
	return srv
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	span, sctx, err := servers.StartSpanFromServer(
		ctx, s.srvContext, "schedule-compile")
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
	return s.scheduler.Schedule(
		logkc.ContextWithLog(ctx, s.lg), req)
}

func (s *schedulerServer) ConnectAgent(
	srv types.Scheduler_ConnectAgentServer,
) error {
	lg := s.lg
	ctx := srv.Context()
	tracer := tracing.TracerFromContext(s.srvContext)
	if err := s.scheduler.AgentConnected(tracing.ContextWithTracer(ctx, tracer)); err != nil {
		return err
	}

	go func() {
		for {
			metadata, err := srv.Recv()
			if err != nil {
				lg.Debug(err)
				return
			}
			if err := s.scheduler.SetToolchains(ctx, metadata.Toolchains.GetItems()); err != nil {
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

	lg.Info(types.Scheduler.Color().Add("Consumerd connected"))

	// add logic here maybe

	// s.monitor.AgentConnected(agent)
	<-srv.Context().Done()

	lg.Info(types.Scheduler.Color().Add("Consumerd disconnected"))
	return nil
}
