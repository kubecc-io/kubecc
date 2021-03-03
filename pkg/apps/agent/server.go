package agent

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/common"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentServer struct {
	types.UnimplementedAgentServer

	AgentServerOptions

	srvContext      context.Context
	executor        run.Executor
	lg              *zap.SugaredLogger
	tcStore         *toolchains.Store
	tcRunStore      *run.ToolchainRunnerStore
	metricsProvider metrics.Provider
}

type AgentServerOptions struct {
	toolchainFinders []toolchains.FinderWithOptions
	toolchainRunners []run.StoreAddFunc
	schedulerClient  types.SchedulerClient
	monitorClient    types.InternalMonitorClient
	usageLimits      *types.UsageLimits
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

func WithToolchainRunners(args ...run.StoreAddFunc) agentServerOption {
	return func(o *AgentServerOptions) {
		o.toolchainRunners = args
	}
}

func WithSchedulerClient(client types.SchedulerClient) agentServerOption {
	return func(o *AgentServerOptions) {
		o.schedulerClient = client
	}
}

func WithMonitorClient(client types.InternalMonitorClient) agentServerOption {
	return func(o *AgentServerOptions) {
		o.monitorClient = client
	}
}

func WithUsageLimits(usageLimits *types.UsageLimits) agentServerOption {
	return func(o *AgentServerOptions) {
		o.usageLimits = usageLimits
	}
}

func NewAgentServer(
	ctx context.Context,
	opts ...agentServerOption,
) *AgentServer {
	options := AgentServerOptions{
		toolchainFinders: []toolchains.FinderWithOptions{
			{
				Finder: cc.CCFinder{},
			},
		},
	}
	options.Apply(opts...)

	runStore := run.NewToolchainRunnerStore()
	for _, add := range options.toolchainRunners {
		add(runStore)
	}

	srv := &AgentServer{
		AgentServerOptions: options,
		srvContext:         ctx,
		lg:                 meta.Log(ctx),
		tcStore:            toolchains.Aggregate(ctx, options.toolchainFinders...),
		tcRunStore:         runStore,
		executor:           run.NewQueuedExecutor(run.WithUsageLimits(options.usageLimits)),
	}
	return srv
}

func (s *AgentServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
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

	resp, err := runner.RecvRemote().Run(run.Contexts{
		ServerContext: s.srvContext,
		ClientContext: sctx,
	}, s.executor, req)
	return resp.(*types.CompileResponse), err
}

func (s *AgentServer) postAlive() {
	s.metricsProvider.Post(&common.Alive{})
}

func (s *AgentServer) postQueueParams() {
	qp := &common.QueueParams{}
	s.executor.CompleteQueueParams(qp)
	s.metricsProvider.Post(qp)
}

func (s *AgentServer) postTaskStatus() {
	ts := &common.TaskStatus{}
	s.executor.CompleteTaskStatus(ts)
	s.metricsProvider.Post(ts)
}

func (s *AgentServer) postQueueStatus() {
	qs := &common.QueueStatus{}
	s.executor.CompleteQueueStatus(qs)
	s.metricsProvider.Post(qs)
}

func (s *AgentServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.metricsProvider = metrics.NewMonitorProvider(s.srvContext, s.monitorClient)
	s.postAlive()
	s.postQueueParams()

	timer := tools.NewJitteredTimer(time.Second/6, 2.0)
	go func() {
		for {
			<-timer
			s.postTaskStatus()
			s.postQueueStatus()
		}
	}()
}

func (s *AgentServer) SetUsageLimits(
	ctx context.Context,
	usageLimits *types.UsageLimits,
) (*types.Empty, error) {
	s.executor.(*run.QueuedExecutor).SetUsageLimits(usageLimits)
	s.usageLimits = usageLimits
	s.postQueueParams()
	return &types.Empty{}, nil
}

func (s *AgentServer) HandleStream(stream grpc.ClientStream) error {
	err := stream.SendMsg(&types.Metadata{
		Toolchains: &types.Toolchains{
			Items: s.tcStore.ItemsList(),
		},
	})
	if err != nil {
		if errors.Is(err, io.EOF) {
			return stream.RecvMsg(nil)
		}
		return err
	}
	select {
	case err := <-servers.EmptyServerStreamDone(s.srvContext, stream):
		return err
	case <-s.srvContext.Done():
		return nil
	}
}

func (s *AgentServer) TryConnect() (grpc.ClientStream, error) {
	return s.schedulerClient.ConnectAgent(s.srvContext)
}

func (s *AgentServer) Target() string {
	return "scheduler"
}
