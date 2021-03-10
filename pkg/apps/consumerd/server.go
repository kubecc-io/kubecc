package consumerd

import (
	"context"
	"io/fs"
	"time"

	cdmetrics "github.com/cobalt77/kubecc/pkg/apps/consumerd/metrics"
	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/common"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type consumerdServer struct {
	types.UnimplementedConsumerdServer

	srvContext context.Context
	lg         *zap.SugaredLogger

	tcRunStore          *run.ToolchainRunnerStore
	tcStore             *toolchains.Store
	storeUpdateCh       chan struct{}
	schedulerClient     types.SchedulerClient
	metricsProvider     metrics.Provider
	connection          *grpc.ClientConn
	localExecutor       run.Executor
	remoteExecutor      run.Executor
	remoteOnly          bool
	numConsumers        *atomic.Int32
	localTasksCompleted *atomic.Int64
}

type ConsumerdServerOptions struct {
	toolchainFinders    []toolchains.FinderWithOptions
	toolchainRunners    []run.StoreAddFunc
	schedulerClient     types.SchedulerClient
	monitorClient       types.InternalMonitorClient
	schedulerConnection *grpc.ClientConn
	usageLimits         *types.UsageLimits
}

type consumerdServerOption func(*ConsumerdServerOptions)

func (o *ConsumerdServerOptions) Apply(opts ...consumerdServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithToolchainFinders(args ...toolchains.FinderWithOptions) consumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.toolchainFinders = args
	}
}

func WithToolchainRunners(args ...run.StoreAddFunc) consumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.toolchainRunners = args
	}
}

func WithSchedulerClient(
	client types.SchedulerClient,
	cc *grpc.ClientConn,
) consumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.schedulerClient = client
		o.schedulerConnection = cc
	}
}

// Note this accepts an InternalMonitorClient even though consumerd runs
// outside the cluster.
func WithMonitorClient(
	client types.InternalMonitorClient,
) consumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.monitorClient = client
	}
}

func WithUsageLimits(cpuConfig *types.UsageLimits) consumerdServerOption {
	return func(o *ConsumerdServerOptions) {
		o.usageLimits = cpuConfig
	}
}

func NewConsumerdServer(
	ctx context.Context,
	opts ...consumerdServerOption,
) *consumerdServer {
	options := ConsumerdServerOptions{
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
	srv := &consumerdServer{
		srvContext:          ctx,
		lg:                  meta.Log(ctx),
		tcStore:             toolchains.Aggregate(ctx, options.toolchainFinders...),
		tcRunStore:          runStore,
		localExecutor:       run.NewQueuedExecutor(run.WithUsageLimits(options.usageLimits)),
		remoteExecutor:      run.NewDelegatingExecutor(),
		storeUpdateCh:       make(chan struct{}, 1),
		numConsumers:        atomic.NewInt32(0),
		localTasksCompleted: atomic.NewInt64(0),
	}
	if options.schedulerClient != nil {
		srv.schedulerClient = options.schedulerClient
		srv.connection = options.schedulerConnection
	}
	if options.monitorClient != nil {
		srv.metricsProvider = metrics.NewMonitorProvider(ctx, options.monitorClient,
			metrics.Buffered|metrics.Discard)
	} else {
		srv.metricsProvider = metrics.NewNoopProvider()
	}
	return srv
}

func (c *consumerdServer) schedulerConnected() bool {
	return c.schedulerClient != nil &&
		c.connection.GetState() == connectivity.Ready
}

func (c *consumerdServer) applyToolchainToReq(req *types.RunRequest) error {
	path := req.GetPath()
	if path == "" {
		return status.Error(codes.InvalidArgument, "No compiler path given")
	}
	tc, err := c.tcStore.Find(path)
	defer func(r *types.RunRequest) {
		r.Compiler = &types.RunRequest_Toolchain{
			Toolchain: tc,
		}
	}(req)
	sendUpdate := func() {
		select {
		case c.storeUpdateCh <- struct{}{}:
		default:
		}
	}

	if err != nil {
		// Add a new toolchain
		c.lg.Info("Consumer sent unknown toolchain; attempting to add it")
		tc, err = c.tcStore.Add(path, toolchains.ExecQuerier{})
		if err != nil {
			c.lg.With(zap.Error(err)).Error("Could not add toolchain")
			return status.Error(codes.InvalidArgument,
				errors.WithMessage(err, "Could not add toolchain").Error())
		}
		defer sendUpdate()
		c.lg.With("compiler", tc.Executable).Info("New toolchain added")
		return nil
	}

	// err and updated are independent
	err, updated := c.tcStore.UpdateIfNeeded(tc)
	if updated {
		defer sendUpdate()
	}
	if err != nil {
		// The toolchain was updated and is no longer valid
		c.lg.With(
			"compiler", tc.Executable,
			zap.Error(err),
		).Error("Error when updating toolchain")
		if errors.As(err, &fs.PathError{}) {
			return status.Error(codes.NotFound,
				errors.WithMessage(err, "Compiler no longer exists").Error())
		}
		return status.Error(codes.InvalidArgument,
			errors.WithMessage(err, "Toolchain is no longer valid").Error())
	}
	return nil
}

func (s *consumerdServer) postAlive() {
	s.metricsProvider.Post(&common.Alive{})
}

func (s *consumerdServer) postQueueParams() {
	qp := &common.QueueParams{}
	s.localExecutor.CompleteQueueParams(qp)
	s.metricsProvider.Post(qp)
}

func (s *consumerdServer) postTaskStatus() {
	ts := &common.TaskStatus{}
	s.localExecutor.CompleteTaskStatus(ts)  // Complete Running and Queued
	s.remoteExecutor.CompleteTaskStatus(ts) // Complete Delegated
	s.metricsProvider.Post(ts)
}

func (s *consumerdServer) postQueueStatus() {
	qs := &common.QueueStatus{}
	s.localExecutor.CompleteQueueStatus(qs)
	s.metricsProvider.Post(qs)
}

func (s *consumerdServer) postTotals() {
	s.metricsProvider.Post(&cdmetrics.LocalTasksCompleted{
		Total: s.localTasksCompleted.Load(),
	})
}

func (s *consumerdServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.postQueueParams()

	slowTimer := util.NewJitteredTimer(5*time.Second, 0.25)
	go func() {
		for {
			<-slowTimer
			s.postQueueParams()
			s.postTotals()
		}
	}()

	fastTimer := util.NewJitteredTimer(time.Second/6, 2.0)
	go func() {
		for {
			<-fastTimer
			s.postTaskStatus()
			s.postQueueStatus()
		}
	}()
}

func (c *consumerdServer) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	if err := meta.CheckContext(ctx); err != nil {
		return nil, err
	}

	c.lg.Debug("Running request")
	err := c.applyToolchainToReq(req)
	if err != nil {
		return nil, err
	}
	// todo
	// rootContext := tracing.ContextWithTracer(ctx, tracer)
	// for _, env := range req.Env {
	// 	spl := strings.Split(env, "=")
	// 	if len(spl) == 2 && spl[0] == "KUBECC_MAKE_PID" {
	// 		pid, err := strconv.Atoi(spl[1])
	// 		if err != nil {
	// 			c.lg.Debug("Invalid value for KUBECC_MAKE_PID, should be a number")
	// 			break
	// 		}
	// 		rootContext = tracing.PIDSpanContext(tracer, pid)
	// 	}
	// }

	span, sctx, err := servers.StartSpanFromServer(ctx, "run")
	if err != nil {
		c.lg.Error(err)
	} else {
		ctx = sctx
		defer span.Finish()
	}

	if req.UID == 0 || req.GID == 0 {
		return nil, status.Error(codes.InvalidArgument,
			"UID or GID cannot be 0")
	}

	runner, err := c.tcRunStore.Get(req.GetToolchain().Kind)
	if err != nil {
		return nil, status.Error(codes.Unavailable,
			"No toolchain runner available")
	}

	ap := runner.NewArgParser(c.srvContext, req.Args)
	ap.Parse()

	canRunRemote := ap.CanRunRemote()
	if !c.schedulerConnected() {
		c.lg.Info("Running local, scheduler disconnected")
		canRunRemote = false
	}
	if !c.remoteOnly && c.localExecutor.Status() == types.Available {
		c.lg.Info("Running local, not at capacity yet")
		canRunRemote = false
	}

	if !canRunRemote {
		defer c.localTasksCompleted.Inc()
		resp, err := runner.RunLocal(ap).Run(run.Contexts{
			ServerContext: c.srvContext,
			ClientContext: ctx,
		}, c.localExecutor, req)
		if err != nil {
			return nil, err
		}
		return resp.(*types.RunResponse), nil
	} else {
		resp, err := runner.SendRemote(ap, c.schedulerClient).Run(run.Contexts{
			ServerContext: c.srvContext,
			ClientContext: ctx,
		}, c.remoteExecutor, req)
		if err != nil {
			return nil, err
		}
		return resp.(*types.RunResponse), nil
	}
}

func (c *consumerdServer) HandleStream(stream grpc.ClientStream) error {
	select {
	case c.storeUpdateCh <- struct{}{}:
	default:
	}
	for {
		select {
		case <-c.storeUpdateCh:
			copiedItems := []*types.Toolchain{}
			for tc := range c.tcStore.Items() {
				copiedItems = append(copiedItems, proto.Clone(tc).(*types.Toolchain))
			}
			err := stream.SendMsg(&types.Metadata{
				Toolchains: &types.Toolchains{
					Items: copiedItems,
				},
			})
			if err != nil {
				c.lg.With(
					zap.Error(err),
				).Error("Error sending updated toolchains to scheduler")
				return err
			}
		case err := <-servers.EmptyServerStreamDone(c.srvContext, stream):
			return err
		case <-c.srvContext.Done():
			return nil
		}
	}
}

func (c *consumerdServer) TryConnect() (grpc.ClientStream, error) {
	return c.schedulerClient.ConnectConsumerd(c.srvContext)
}

func (c *consumerdServer) Target() string {
	return "scheduler"
}
