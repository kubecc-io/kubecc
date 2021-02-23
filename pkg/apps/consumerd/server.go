package consumerd

import (
	"context"
	"io/fs"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/cpuconfig"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

type consumerdServer struct {
	types.ConsumerdServer

	srvContext context.Context
	lg         *zap.SugaredLogger

	tcRunStore      *run.ToolchainRunnerStore
	tcStore         *toolchains.Store
	schedulerClient types.SchedulerClient
	connection      *grpc.ClientConn
	localExecutor   run.Executor
	remoteExecutor  run.Executor
	remoteOnly      bool
}

type ConsumerdServerOptions struct {
	toolchainFinders    []toolchains.FinderWithOptions
	toolchainRunners    []run.StoreAddFunc
	schedulerClient     types.SchedulerClient
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

	if options.usageLimits == nil {
		options.usageLimits = cpuconfig.Default()
	}

	runStore := run.NewToolchainRunnerStore()
	for _, add := range options.toolchainRunners {
		add(runStore)
	}
	srv := &consumerdServer{
		srvContext:     ctx,
		lg:             logkc.LogFromContext(ctx),
		tcStore:        toolchains.Aggregate(ctx, options.toolchainFinders...),
		tcRunStore:     runStore,
		localExecutor:  run.NewQueuedExecutor(run.WithUsageLimits(options.usageLimits)),
		remoteExecutor: run.NewUnqueuedExecutor(),
		remoteOnly:     viper.GetBool("remoteOnly"),
	}
	if options.schedulerClient != nil {
		srv.schedulerClient = options.schedulerClient
		srv.connection = options.schedulerConnection
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
	if err != nil {
		// Add a new toolchain
		c.lg.Info("Consumer sent unknown toolchain; attempting to add it")
		tc, err = c.tcStore.Add(path, toolchains.ExecQuerier{})
		if err != nil {
			c.lg.With(zap.Error(err)).Error("Could not add toolchain")
			return status.Error(codes.InvalidArgument,
				errors.WithMessage(err, "Could not add toolchain").Error())
		}
		c.lg.With("compiler", tc.Executable).Info("New toolchain added")
	} else if err := c.tcStore.UpdateIfNeeded(tc); err != nil {
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
	req.Compiler = &types.RunRequest_Toolchain{
		Toolchain: tc,
	}
	return nil
}

func (c *consumerdServer) Run(
	ctx context.Context,
	req *types.RunRequest,
) (*types.RunResponse, error) {
	c.lg.Debug("Running request")
	err := c.applyToolchainToReq(req)
	if err != nil {
		return nil, err
	}
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

	span, sctx, err := servers.StartSpanFromServer(ctx, c.srvContext, "run")
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

func (c *consumerdServer) ConnectToRemote() {
	addr := viper.GetString("schedulerAddress")
	if addr == "" {
		c.lg.Debug("Remote compilation unavailable: scheduler address not configured")
		return
	}
	cc, err := servers.Dial(c.srvContext, addr, servers.WithTLS(viper.GetBool("tls")))
	if err != nil {
		c.lg.With(zap.Error(err)).Info("Remote compilation unavailable")
	} else {
		c.connection = cc
		c.schedulerClient = types.NewSchedulerClient(cc)
	}
}
