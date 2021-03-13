package agent

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TaskStreamManager struct {
	srvContext      context.Context
	lg              *zap.SugaredLogger
	schedulerClient types.SchedulerClient
	tcStore         *toolchains.Store
	tcRunStore      *run.ToolchainRunnerStore
	executor        run.Executor
}

func (s *TaskStreamManager) HandleStream(stream grpc.ClientStream) error {
	s.lg.Info("Streaming tasks from scheduler")
	defer s.lg.Warn("Task stream closed")
	streamCtx := stream.Context()
	for {
		compileRequest := &types.CompileRequest{}
		err := stream.RecvMsg(compileRequest)
		if err != nil {
			return err
		}
		go s.Compile(streamCtx, compileRequest)
	}
}

func (s *TaskStreamManager) TryConnect() (grpc.ClientStream, error) {
	return s.schedulerClient.ConnectAgent(s.srvContext)
}

func (s *TaskStreamManager) Target() string {
	return "scheduler"
}

func (s *TaskStreamManager) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	s.lg.Debug("Handling compile request")
	if err := meta.CheckContext(ctx); err != nil {
		return nil, err
	}

	span, sctx, err := servers.StartSpanFromServer(ctx, "compile")
	if err != nil {
		s.lg.Error(err)
	} else {
		defer span.Finish()
	}

	runner, err := s.tcRunStore.Get(req.GetToolchain().Kind)
	if err != nil {
		return nil, status.Error(codes.Unavailable,
			"No toolchain runner available")
	}

	tc, err := s.tcStore.TryMatch(req.GetToolchain())
	if err != nil {
		return nil, status.Error(codes.Unavailable,
			err.Error())
	}

	// Swap remote toolchain with the local toolchain in case the executable
	// path is different locally
	req.Toolchain = tc
	resp, err := runner.RecvRemote().Run(run.Contexts{
		ServerContext: s.srvContext,
		ClientContext: sctx,
	}, s.executor, req)
	if err != nil {
		s.lg.With(
			zap.Error(err),
		).Error("Error from remote runner")
	}
	return resp.(*types.CompileResponse), err
}
