package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/cpuconfig"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentServer struct {
	types.AgentServer
	srvContext context.Context
	lg         *zap.SugaredLogger

	toolchains *toolchains.Store

	executor        *run.Executor
	cpuConfig       *types.CpuConfig
	schedulerClient types.SchedulerClient

	queueStatus   types.QueueStatus
	queueStatusCh chan types.QueueStatus
}

type AgentServerOptions struct {
	toolchainOptions []toolchains.FindOption
}

type agentServerOption func(*AgentServerOptions)

func (o *AgentServerOptions) Apply(opts ...agentServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithToolchainArgs(args ...toolchains.FindOption) agentServerOption {
	return func(o *AgentServerOptions) {
		o.toolchainOptions = args
	}
}

func NewAgentServer(
	ctx context.Context,
	opts ...agentServerOption,
) *AgentServer {
	options := AgentServerOptions{}
	options.Apply(opts...)

	srv := &AgentServer{
		srvContext:  ctx,
		lg:          logkc.LogFromContext(ctx),
		toolchains:  toolchains.FindToolchains(ctx, options.toolchainOptions...),
		executor:    run.NewExecutor(run.WithCpuConfig(cpuconfig.Default())),
		queueStatus: types.Available,
	}
	return srv
}

func (s *AgentServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	s.updateQueueStatus(s.executor.Status())

	if runner, ok := toolchainRunners[req.Toolchain.Kind]; ok {
		return runner.Run(Contexts{
			ServerContext: s.srvContext,
			ClientContext: ctx,
		}, s.executor, req)
	}
	return nil, status.Error(codes.Unimplemented,
		"No implementation available for the given Toolchain")
}

func (s *AgentServer) updateQueueStatus(stat types.QueueStatus) {
	if s.queueStatus != stat {
		s.queueStatus = stat
		select {
		case s.queueStatusCh <- stat:
		default:
		}
	}
}

func (s *AgentServer) RunSchedulerClient(ctx context.Context, a types.AgentServer) {
	cc, err := grpc.Dial(
		fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090",
			cluster.GetNamespace()),
		grpc.WithInsecure())
	if err != nil {
		s.lg.With(zap.Error(err)).Fatal("Error dialing scheduler")
	}
	s.schedulerClient = types.NewSchedulerClient(cc)
	for {
		s.lg.Info("Starting connection to the scheduler")
		stream, err := s.schedulerClient.ConnectAgent(ctx, grpc.WaitForReady(true))
		if err != nil {
			s.lg.With(zap.Error(err)).Error("Error connecting to scheduler. Reconnecting in 5 seconds")
			time.Sleep(5 * time.Second)
		}
		err = stream.Send(&types.Metadata{
			Contents: &types.Metadata_Toolchains{
				Toolchains: &types.Toolchains{
					Items: s.toolchains.ItemsList(),
				},
			},
		})
		if err != nil {
			s.lg.Error(err)
		}
		s.lg.Info("Connected to the scheduler")

		streamClosed := make(chan struct{})
		go func() {
			for {
				_, err := stream.Recv()
				if err != nil {
					s.lg.With(zap.Error(err)).Error("Connection lost, reconnecting...")
				}
				close(streamClosed)
				return
			}
		}()

		select {
		case stat := <-s.queueStatusCh:
			s.lg.Info("Sending queue status update: %s",
				types.QueueStatus_name[int32(stat)])
			err := stream.Send(&types.Metadata{
				Contents: &types.Metadata_QueueStatus{
					QueueStatus: stat,
				},
			})
			if err != nil {
				s.lg.Error(err)
			}
		case <-streamClosed:
		}
	}
}

func (s *AgentServer) GetCpuConfig(
	ctx context.Context,
	_ *types.Empty,
) (*types.CpuConfig, error) {
	return s.cpuConfig, nil
}

func (s *AgentServer) SetCpuConfig(
	ctx context.Context,
	cfg *types.CpuConfig,
) (*types.Empty, error) {
	s.executor.SetCpuConfig(cfg)
	s.cpuConfig = cfg
	return &types.Empty{}, nil
}
