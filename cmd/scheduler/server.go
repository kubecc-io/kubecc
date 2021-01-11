package main

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"
)

type schedulerServer struct {
	types.SchedulerServer

	scheduler AgentScheduler
	watcher   *Monitor
}

func NewSchedulerServer() *schedulerServer {
	scheduler, err := NewRoundRobinScheduler()
	if err != nil {
		log.Fatal(err)
	}
	return &schedulerServer{
		scheduler: scheduler,
		watcher:   NewMonitor(),
	}
}

func (s *schedulerServer) AtCapacity(
	ctx context.Context,
	req *types.Empty,
) (*wrappers.BoolValue, error) {
	peer, ok := peer.FromContext(ctx)
	if ok {
		log.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	return &wrappers.BoolValue{Value: false}, nil
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if ok {
		log.With("peer", peer.Addr.String()).Info("Schedule requested")
	}
	task, err := s.scheduler.Schedule(req)
	if err != nil {
		log.With(zap.Error(err)).Info("=> Schedule failed")
		return nil, err
	}
	log.Info("=> Schedule OK")
	return s.watcher.Wait(task)
}

func (s *schedulerServer) Connect(
	srv types.Scheduler_ConnectServer,
) error {
	agent, err := NewAgentFromContext(srv.Context())
	if err != nil {
		log.With(zap.Error(err)).Error("Error identifying agent using context")
		return nil
	}
	lg := log.With("agent", agent)

	lg.Info("Agent connected")

	s.watcher.AgentConnected(agent)
	<-srv.Context().Done()

	lg.Info("Agent disconnected")
	return nil
}
