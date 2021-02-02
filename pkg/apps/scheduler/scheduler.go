package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
)

type AgentScheduler interface {
	Schedule(context.Context, *types.CompileRequest) (*types.CompileResponse, error)
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
) (*types.CompileResponse, error) {
	lg := lll.LogFromContext(ctx)

	if s.agentClient == nil {
		lg.Info("Starting resolver")
		cc, err := s.resolver.Dial()
		if err != nil {
			return nil, err
		}
		s.agentClient = types.NewAgentClient(cc)
	}
	lg.Info("Scheduling")
	for {
		response, err := s.agentClient.Compile(ctx, req, grpc.UseCompressor(gzip.Name))
		if status.Code(err) == codes.Unavailable {
			lg.Info("Agent rejected task, re-scheduling...")
			continue
		}
		if err != nil {
			lg.With(zap.Error(err)).Error("Error from agent")
			return nil, err
		}
		return response, nil
	}
}
