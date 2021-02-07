package scheduler

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
)

type AgentScheduler struct {
	agentClient types.AgentClient
	resolver    AgentResolver

	ctx context.Context
	lg  *zap.SugaredLogger
}

func NewAgentScheduler(ctx context.Context) *AgentScheduler {
	return &AgentScheduler{
		ctx: ctx,
		lg:  logkc.LogFromContext(ctx),
	}
}

func (s *AgentScheduler) Schedule(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	if s.agentClient == nil {
		s.lg.Info("Starting resolver")
		cc, err := s.resolver.Dial()
		if err != nil {
			return nil, err
		}
		s.agentClient = types.NewAgentClient(cc)
	}
	s.lg.Info("Scheduling")
	for {
		response, err := s.agentClient.Compile(ctx, req, grpc.UseCompressor(gzip.Name))
		if status.Code(err) == codes.Unavailable {
			s.lg.Info("Agent rejected task, re-scheduling...")
			continue
		}
		if err != nil {
			s.lg.With(zap.Error(err)).Error("Error from agent")
			return nil, err
		}
		return response, nil
	}
}
