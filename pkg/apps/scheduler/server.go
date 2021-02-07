package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.SchedulerServer

	lg        *zap.SugaredLogger
	scheduler *AgentScheduler
	monitor   *Monitor
}

func NewSchedulerServer(ctx context.Context) *schedulerServer {
	srv := &schedulerServer{
		monitor:   NewMonitor(),
		scheduler: NewAgentScheduler(ctx),
		lg:        logkc.LogFromContext(ctx),
	}
	return srv
}

func (s *schedulerServer) AtCapacity(
	ctx context.Context,
	req *types.Empty,
) (*wrappers.BoolValue, error) {
	// this isnt used
	peer, ok := peer.FromContext(ctx)
	if ok {
		s.lg.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	return &wrappers.BoolValue{Value: false}, nil
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
	// task, err := s.atomicScheduler().Schedule(ctx, req)
	// if err != nil {
	// 	logkc.With(zap.Error(err)).Info("=> Schedule failed")
	// 	return nil, err
	// }
	// logkc.Info("=> Schedule OK")
	// return s.monitor.Wait(task)
}

func (s *schedulerServer) ConnectAgent(
	srv types.Scheduler_ConnectAgentServer,
) error {
	agent, err := NewAgentFromContext(srv.Context())
	if err != nil {
		s.lg.With(zap.Error(err)).Error("Error identifying agent using context")
		return nil
	}
	lg := s.lg.With("agent", agent)

	lg.Info("Agent connected")

	// add logic here maybe

	s.monitor.AgentConnected(agent)
	<-srv.Context().Done()

	lg.Info("Agent disconnected")
	return nil
}

func (s *schedulerServer) ConnectConsumerd(
	srv types.Scheduler_ConnectConsumerdServer,
) error {
	s.lg.Info("Consumerd connected")

	// add logic here maybe

	// s.monitor.ConsumerdConnected(agent)
	<-srv.Context().Done()

	s.lg.Info("Consumerd disconnected")
	return nil
}
