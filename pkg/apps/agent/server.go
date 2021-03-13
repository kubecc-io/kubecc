package agent

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/common"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type AgentServer struct {
	AgentServerOptions

	srvContext      context.Context
	executor        run.Executor
	lg              *zap.SugaredLogger
	tcStore         *toolchains.Store
	metricsProvider metrics.Provider
	taskStreamMgr   *TaskStreamManager
}

type AgentServerOptions struct {
	toolchainFinders []toolchains.FinderWithOptions
	toolchainRunners []run.StoreAddFunc
	schedulerClient  types.SchedulerClient
	monitorClient    types.MonitorClient
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

func WithMonitorClient(client types.MonitorClient) agentServerOption {
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
	options := AgentServerOptions{}
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
		executor:           run.NewQueuedExecutor(run.WithUsageLimits(options.usageLimits)),
	}
	srv.taskStreamMgr = &TaskStreamManager{
		srvContext:      ctx,
		lg:              meta.Log(ctx),
		schedulerClient: options.schedulerClient,
		tcStore:         srv.tcStore,
		tcRunStore:      runStore,
		executor:        srv.executor,
	}
	mgr := servers.NewStreamManager(ctx, srv.taskStreamMgr)
	go mgr.Run()

	return srv
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
	s.metricsProvider = metrics.NewMonitorProvider(s.srvContext, s.monitorClient,
		metrics.Buffered|metrics.Discard)
	s.postQueueParams()

	fastTimer := util.NewJitteredTimer(time.Second/6, 2.0)
	go func() {
		for {
			<-fastTimer
			s.postTaskStatus()
			s.postQueueStatus()
		}
	}()

	slowTimer := util.NewJitteredTimer(5*time.Second, 0.5)
	go func() {
		for {
			<-slowTimer
			s.postQueueParams()
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
	s.lg.Info("Streaming metadata to scheduler")
	defer s.lg.Warn("Stream closed")
	err := stream.SendMsg(&types.Metadata{
		Toolchains: &types.Toolchains{
			Items: s.tcStore.ItemsList(),
		},
	})
	if err != nil {
		if errors.Is(err, io.EOF) {
			return stream.RecvMsg(nil)
		}
		s.lg.Error(err)
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
