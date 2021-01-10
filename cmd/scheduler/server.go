package main

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/types"
)

type schedulerServer struct {
	types.SchedulerServer

	scheduler AgentScheduler
	watcher   *AgentWatcher
}

func NewSchedulerServer() *schedulerServer {
	scheduler, err := NewRoundRobinScheduler()
	if err != nil {
		log.Fatal(err)
	}
	return &schedulerServer{
		scheduler: scheduler,
		watcher:   NewAgentWatcher(),
	}
}

func (s *schedulerServer) Schedule(
	ctx context.Context,
	req *types.ScheduleRequest,
) (*types.ScheduleResponse, error) {
	agent, err := s.watcher.GetAgentInfo(ctx)
	if err != nil {
		return nil, err
	}
	log.With("agent", agent).Info("Schedule requested")
	return &types.ScheduleResponse{}, nil
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	info, err := s.watcher.GetAgentInfo(ctx)
	if err != nil {
		return nil, err
	}
	task, err := s.scheduler.Schedule(req)
	if err != nil {
		return nil, err
	}
	return s.watcher.Wait(info, task)
}

func (s *schedulerServer) Connect(
	srv types.Scheduler_ConnectServer,
) error {
	agent, err := NewAgentFromContext(srv.Context())
	if err != nil {
		return nil
	}
	lg := log.With("agent", agent)

	lg.Info("Agent connected")

	s.watcher.WatchAgent(agent)
	<-srv.Context().Done()

	lg.Info("Agent disconnected")
	return nil
}
