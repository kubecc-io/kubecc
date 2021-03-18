package test

import (
	"context"
	"net"
	"time"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil/toolchain"
	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/apps/cachesrv"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/storage"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	mapset "github.com/deckarep/golang-set"
	"github.com/imdario/mergo"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	reftypes "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/test/bufconn"
)

type Environment struct {
	defaultConfig  config.KubeccSpec
	envContext     context.Context
	envCancel      context.CancelFunc
	listener       *bufconn.Listener
	server         *grpc.Server
	agentCount     *atomic.Int32
	consumerdCount *atomic.Int32
}

var (
	bufferSize = 1024 * 1024
)

type SpawnOptions struct {
	config config.KubeccSpec
}

type SpawnOption func(*SpawnOptions)

func (o *SpawnOptions) Apply(opts ...SpawnOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithConfig(cfg interface{}) SpawnOption {
	return func(o *SpawnOptions) {
		switch conf := cfg.(type) {
		case config.KubeccSpec:
			o.config = conf
		case config.AgentSpec:
			o.config = config.KubeccSpec{Agent: conf}
		case config.ConsumerdSpec:
			o.config = config.KubeccSpec{Consumerd: conf}
		case config.SchedulerSpec:
			o.config = config.KubeccSpec{Scheduler: conf}
		case config.CacheSpec:
			o.config = config.KubeccSpec{Cache: conf}
		case config.MonitorSpec:
			o.config = config.KubeccSpec{Monitor: conf}
		}
	}
}

func (e *Environment) SpawnAgent(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName(string('a'+e.agentCount.Load())),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	e.agentCount.Inc()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		e.agentCount.Dec()
	}()

	options := []agent.AgentServerOption{
		agent.WithUsageLimits(&metrics.UsageLimits{
			ConcurrentProcessLimit:  int32(cfg.Agent.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: cfg.Agent.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   cfg.Agent.UsageLimits.QueueRejectMultiplier,
		}),
		agent.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		agent.WithToolchainRunners(testtoolchain.AddToStore),
		agent.WithMonitorClient(types.NewMonitorClient(e.Dial(ctx))),
		agent.WithSchedulerClient(types.NewSchedulerClient(e.Dial(ctx))),
	}

	agentSrv := agent.NewAgentServer(ctx, options...)
	mgr := servers.NewStreamManager(ctx, agentSrv)
	go mgr.Run()
	go agentSrv.StartMetricsProvider()
	return ctx, cancel
}

func (e *Environment) SpawnScheduler(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Scheduler,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	options := []scheduler.SchedulerServerOption{
		scheduler.WithMonitorClient(types.NewMonitorClient(e.Dial(ctx))),
		scheduler.WithCacheClient(types.NewCacheClient(e.Dial(ctx))),
	}

	sc := scheduler.NewSchedulerServer(ctx, options...)
	types.RegisterSchedulerServer(e.server, sc)
	go sc.StartMetricsProvider()

	return ctx, cancel
}

func (e *Environment) SpawnConsumerd(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Consumerd,
				logkc.WithName(string('a'+e.consumerdCount.Load())),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	e.consumerdCount.Inc()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		e.consumerdCount.Dec()
	}()

	options := []consumerd.ConsumerdServerOption{
		consumerd.WithUsageLimits(&metrics.UsageLimits{
			ConcurrentProcessLimit:  int32(cfg.Consumerd.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: cfg.Consumerd.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   cfg.Consumerd.UsageLimits.QueueRejectMultiplier,
		}),
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		consumerd.WithToolchainRunners(testtoolchain.AddToStore),
		consumerd.WithMonitorClient(types.NewMonitorClient(e.Dial(ctx))),
		consumerd.WithSchedulerClient(types.NewSchedulerClient(e.Dial(ctx))),
	}

	cd := consumerd.NewConsumerdServer(ctx, options...)
	types.RegisterConsumerdServer(e.server, cd)

	go cd.StartMetricsProvider()

	return ctx, cancel
}

func (e *Environment) SpawnMonitor(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Monitor,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	mon := monitor.NewMonitorServer(ctx, monitor.InMemoryStoreCreator)
	types.RegisterMonitorServer(e.server, mon)

	return ctx, cancel
}

func (e *Environment) SpawnCache(opts ...SpawnOption) (context.Context, context.CancelFunc) {
	so := SpawnOptions{
		config: e.defaultConfig,
	}
	so.Apply(opts...)
	cfg := e.defaultConfig
	if err := mergo.Merge(&cfg, so.config); err != nil {
		panic(err)
	}

	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Cache)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Cache,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	options := []cachesrv.CacheServerOption{
		cachesrv.WithMonitorClient(types.NewMonitorClient(e.Dial(ctx))),
	}

	providers := []storage.StorageProvider{}
	if cfg.Cache.LocalStorage != nil {
		providers = append(providers,
			storage.NewVolatileStorageProvider(ctx, *cfg.Cache.LocalStorage))
	}
	if cfg.Cache.RemoteStorage != nil {
		providers = append(providers,
			storage.NewS3StorageProvider(ctx, *cfg.Cache.RemoteStorage))
	}
	options = append(options, cachesrv.WithStorageProvider(
		storage.NewChainStorageProvider(ctx, providers...),
	))

	cacheSrv := cachesrv.NewCacheServer(ctx, cfg.Cache, options...)

	types.RegisterCacheServer(e.server, cacheSrv)
	go cacheSrv.StartMetricsProvider()

	return ctx, cancel
}

func DefaultConfig() config.KubeccSpec {
	return config.KubeccSpec{
		Global: config.GlobalSpec{
			LogLevel: "debug",
		},
		Agent: config.AgentSpec{
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit:  32,
				QueuePressureMultiplier: 1.0,
				QueueRejectMultiplier:   2.0,
			},
		},
		Cache: config.CacheSpec{
			LocalStorage: &config.LocalStorageSpec{
				Limits: config.StorageLimitsSpec{
					Memory: "1Gi",
				},
			},
		},
		Consumerd: config.ConsumerdSpec{
			DisableTLS: true,
			UsageLimits: config.UsageLimitsSpec{
				ConcurrentProcessLimit: 20,
			},
		},
	}
}

func NewEnvironment(cfg config.KubeccSpec) *Environment {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)

	return &Environment{
		defaultConfig:  cfg,
		envContext:     ctx,
		envCancel:      cancel,
		listener:       bufconn.Listen(bufferSize),
		server:         servers.NewServer(ctx),
		agentCount:     atomic.NewInt32(0),
		consumerdCount: atomic.NewInt32(0),
	}
}

func NewDefaultEnvironment() *Environment {
	return NewEnvironment(DefaultConfig())
}

func (e *Environment) Serve() {
	reflection.Register(e.server)
	go func() {
		err := e.server.Serve(e.listener)
		if err != nil {
			meta.Log(e.envContext).Error(err)
		}
	}()
}

func (e *Environment) WaitForServices(names []string) {
	c := reftypes.NewServerReflectionClient(e.Dial(e.envContext))
	stream, err := c.ServerReflectionInfo(e.envContext)
	if err != nil {
		panic(err)
	}
	for {
		err := stream.Send(&reftypes.ServerReflectionRequest{
			MessageRequest: &reftypes.ServerReflectionRequest_ListServices{ListServices: ""},
		})
		if err != nil {
			panic(err)
		}
		response, err := stream.Recv()
		if err != nil {
			panic(err)
		}
		list := response.MessageResponse.(*reftypes.ServerReflectionResponse_ListServicesResponse)
		services := mapset.NewSet()
		for _, svc := range list.ListServicesResponse.Service {
			services.Add(svc.GetName())
		}
		values := []interface{}{}
		for _, name := range names {
			values = append(values, name)
		}
		if services.Contains(values...) {
			return
		}
		meta.Log(e.envContext).With(
			zap.Any("have", services),
			zap.Any("want", names),
		).Info("Waiting for services")
		time.Sleep(250 * time.Millisecond)
	}
}

func (e *Environment) Dial(ctx context.Context) *grpc.ClientConn {
	cc, err := servers.Dial(ctx, "bufconn", servers.WithDialOpts(
		grpc.WithContextDialer(
			func(context.Context, string) (net.Conn, error) {
				return e.listener.Dial()
			}),
	))
	if err != nil {
		panic(err)
	}
	return cc
}

func (e *Environment) NewMonitorClient(ctx context.Context) types.MonitorClient {
	return types.NewMonitorClient(e.Dial(ctx))
}

func (e *Environment) NewSchedulerClient(ctx context.Context) types.SchedulerClient {
	return types.NewSchedulerClient(e.Dial(ctx))
}

func (e *Environment) NewCacheClient(ctx context.Context) types.CacheClient {
	return types.NewCacheClient(e.Dial(ctx))
}

func (e *Environment) NewConsumerdClient(ctx context.Context) types.ConsumerdClient {
	return types.NewConsumerdClient(e.Dial(ctx))
}

func (e *Environment) Shutdown() {
	e.envCancel()
}
