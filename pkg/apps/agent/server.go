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

	AgentServerOptions

	executor      run.Executor
	lg            *zap.SugaredLogger
	queueStatus   types.QueueStatus
	queueStatusCh chan types.QueueStatus
	srvContext    context.Context
	toolchains    *toolchains.Store
}

type AgentServerOptions struct {
	toolchainFinders []toolchains.FinderWithOptions
	schedulerClient  types.SchedulerClient
	cpuConfig        *types.CpuConfig
}

type agentServerOption func(*AgentServerOptions)

func (o *AgentServerOptions) Apply(opts ...agentServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithToolchainFinders(args ...toolchains.FinderWithOptions) agentServerOption {
	return func(o *AgentServerOptions) {
		o.toolchainFinders = args
	}
}

func WithSchedulerClient(client types.SchedulerClient) agentServerOption {
	return func(o *AgentServerOptions) {
		o.schedulerClient = client
	}
}

func WithCpuConfig(cpuConfig *types.CpuConfig) agentServerOption {
	return func(o *AgentServerOptions) {
		o.cpuConfig = cpuConfig
	}
}

func NewAgentServer(
	ctx context.Context,
	opts ...agentServerOption,
) *AgentServer {
	options := AgentServerOptions{
		toolchainFinders: []toolchains.FinderWithOptions{
			{
				Finder: toolchains.GccClangFinder{},
			},
		},
	}
	options.Apply(opts...)

	if options.cpuConfig == nil {
		options.cpuConfig = cpuconfig.Default()
	}

	srv := &AgentServer{
		AgentServerOptions: options,
		srvContext:         ctx,
		lg:                 logkc.LogFromContext(ctx),
		toolchains:         toolchains.Aggregate(ctx, options.toolchainFinders...),
		executor:           run.NewQueuedExecutor(run.WithCpuConfig(options.cpuConfig)),
		queueStatus:        types.Available,
	}
	return srv
}

func (s *AgentServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	s.updateQueueStatus(s.executor.Status())

	if runner, ok := toolchainRunners[req.Toolchain.Kind]; ok {
		resp, err := runner.Run(run.Contexts{
			ServerContext: s.srvContext,
			ClientContext: ctx,
		}, s.executor, req)
		return resp.(*types.CompileResponse), err
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

func (s *AgentServer) RunSchedulerClient(ctx context.Context) {
	if s.schedulerClient == nil {
		cc, err := grpc.Dial(
			fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090",
				cluster.GetNamespace()),
			grpc.WithInsecure())
		if err != nil {
			s.lg.With(zap.Error(err)).Fatal("Error dialing scheduler")
		}
		s.schedulerClient = types.NewSchedulerClient(cc)
	}

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

		streamClosed := make(chan error)
		go func() {
			for {
				_, err := stream.Recv()
				streamClosed <- err
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
		case err := <-streamClosed:
			s.lg.With(zap.Error(err)).Error("Connection lost. Reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
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
	s.executor.(*run.QueuedExecutor).SetCpuConfig(cfg)
	s.cpuConfig = cfg
	return &types.Empty{}, nil
}
