package main

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
)

type AgentScheduler interface {
	Schedule(context.Context, *types.CompileRequest) (*CompileTask, error)
}

var (
	schedulers map[string]AgentScheduler = make(map[string]AgentScheduler)
)

func AddScheduler(name string, scheduler AgentScheduler) {
	schedulers[name] = scheduler
}

func GetScheduler(name string) (s AgentScheduler, ok bool) {
	s, ok = schedulers[name]
	return
}

type DefaultScheduler struct {
	AgentScheduler

	agentClient types.AgentClient
	resolver    AgentResolver
}

func (s *DefaultScheduler) Schedule(
	ctx context.Context,
	req *types.CompileRequest,
) (*CompileTask, error) {
	if s.agentClient == nil {
		log.Info("Starting resolver")
		cc, err := s.resolver.Dial()
		if err != nil {
			return nil, err
		}
		s.agentClient = types.NewAgentClient(cc)
	}
	log.Info("Scheduling")
	s.resolver.Dial()
	stream, err := s.agentClient.Compile(ctx, req, grpc.UseCompressor(gzip.Name))
	if err != nil {
		log.With(zap.Error(err)).Error("Error from agent")
		return nil, err
	}
	log.Info("Agent started task")
	return NewCompileTask(stream), nil
}
