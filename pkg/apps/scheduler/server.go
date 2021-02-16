package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.SchedulerServer

	lg        *zap.SugaredLogger
	scheduler *Scheduler
}

func NewSchedulerServer(ctx context.Context) *schedulerServer {
	srv := &schedulerServer{
		scheduler: NewScheduler(ctx),
		lg:        logkc.LogFromContext(ctx),
	}
	return srv
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	span, sctx := opentracing.StartSpanFromContext(ctx, "schedule-compile")
	defer span.Finish()

	peer, ok := peer.FromContext(ctx)
	if ok {
		s.lg.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	return s.scheduler.Schedule(
		logkc.ContextWithLog(sctx, s.lg), req)

}

func (s *schedulerServer) ConnectAgent(
	srv types.Scheduler_ConnectServer,
) error {
	lg := s.lg
	ctx := srv.Context()
	if err := s.scheduler.AgentConnected(ctx); err != nil {
		return err
	}

	go func() {
		for {
			metadata, err := srv.Recv()
			if err != nil {
				lg.Debug(err)
				return
			}
			switch msg := metadata.Contents.(type) {
			case *types.Metadata_QueueStatus:
				if err := s.scheduler.SetQueueStatus(ctx, msg.QueueStatus); err != nil {
					lg.Error(err)
				}
			case *types.Metadata_Toolchains:
				if err := s.scheduler.SetToolchains(ctx, msg.Toolchains.GetItems()); err != nil {
					lg.Error(err)
				}
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func (s *schedulerServer) ConnectConsumerd(
	srv types.Scheduler_ConnectServer,
) error {
	lg := s.lg

	lg.Info("Consumerd connected")

	// add logic here maybe

	// s.monitor.AgentConnected(agent)
	<-srv.Context().Done()

	lg.Info("Consumerd disconnected")
	return nil
}
