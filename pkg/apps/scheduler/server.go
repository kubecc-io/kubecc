package scheduler

import (
	"context"
	"errors"
	"io"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.SchedulerServer

	srvContext meta.Context
	lg         *zap.SugaredLogger
	scheduler  *Scheduler
}

func NewSchedulerServer(
	ctx meta.Context,
	opts ...schedulerOption,
) *schedulerServer {
	srv := &schedulerServer{
		srvContext: ctx,
		lg:         ctx.Log(),
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
	return s.scheduler.Schedule(ctx, req)
}

func (s *schedulerServer) ConnectAgent(
	srv types.Scheduler_ConnectAgentServer,
) error {
	if err := meta.CheckContext(srv.Context()); err != nil {
		return err
	}
	lg := s.lg
	ctx := srv.Context().(meta.Context)
	if err := s.scheduler.AgentConnected(ctx); err != nil {
		return err
	}

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

	lg.Info(types.Scheduler.Color().Add("Consumerd connected"))
	defer lg.Info(types.Scheduler.Color().Add("Consumerd disconnected"))

	go func() {
		for {
			_, err := srv.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					lg.Debug(err)
				} else {
					lg.Error(err)
				}
				return
			}

			// if err := s.scheduler.SetToolchains(
			// 	ctx, metadata.Toolchains.GetItems()); err != nil {
			// 	lg.Error(err)
			// }
		}
	}()

	<-ctx.Done()

	return nil
}
