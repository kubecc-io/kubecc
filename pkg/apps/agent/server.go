package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/cpuconfig"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/mmeta"
	mstat "github.com/cobalt77/kubecc/pkg/metrics/status"
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
	types.AgentServer

	AgentServerOptions

	srvContext      context.Context
	executor        run.Executor
	lg              *zap.SugaredLogger
	tcStore         *toolchains.Store
	tcRunStore      *run.ToolchainRunnerStore
	metricsProvider *metrics.Provider
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

	if options.usageLimits == nil {
		options.usageLimits = cpuconfig.Default()
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
	s.metricsProvider.Post(&mmeta.Alive{})
}

func (s *AgentServer) postQueueParams() {
	qp := &mstat.QueueParams{}
	s.executor.CompleteQueueParams(qp)
	s.metricsProvider.Post(qp)
}

func (s *AgentServer) postTaskStatus() {
	ts := &mstat.TaskStatus{}
	s.executor.CompleteTaskStatus(ts)
	s.metricsProvider.Post(ts)
}

func (s *AgentServer) postQueueStatus() {
	qs := &mstat.QueueStatus{}
	s.executor.CompleteQueueStatus(qs)
	s.metricsProvider.Post(qs)
}

func (s *AgentServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.metricsProvider = metrics.NewProvider(s.srvContext, s.monitorClient)
	s.postAlive()
	s.postQueueParams()
	s.postQueueStatus()

	timer := tools.NewJitteredTimer(time.Second/6, 2.0)
	go func() {
		for {
			<-timer
			s.postTaskStatus()
			s.postQueueStatus()
		}
	}()
}

func (s *AgentServer) RunSchedulerClient() {
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
		stream, err := s.schedulerClient.ConnectAgent(
			s.srvContext, grpc.WaitForReady(true))
		if err != nil {
			s.lg.With(zap.Error(err)).Error("Error connecting to scheduler. Reconnecting in 5 seconds")
			time.Sleep(5 * time.Second)
		}
		err = stream.Send(&types.Metadata{
			Toolchains: &types.Toolchains{
				Items: s.tcStore.ItemsList(),
			},
		})
		if err != nil {
			s.lg.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}
		s.lg.Info(types.Agent.Color().Add("Connected to the scheduler"))

		streamClosed := make(chan error)
		go func() {
			for {
				_, err := stream.Recv()
				streamClosed <- err
				close(streamClosed)
				return
			}
		}()

		<-streamClosed
		s.lg.With(zap.Error(err)).Warn("Connection lost. Reconnecting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
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
