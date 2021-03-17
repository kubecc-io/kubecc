package test

import (
	"context"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil/toolchain"
	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/apps/cachesrv"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/clients"
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
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type Environment struct {
	Config         *config.KubeccSpec
	envContext     context.Context
	envCancel      context.CancelFunc
	portMapping    map[string]*bufconn.Listener
	agentCount     *atomic.Int32
	consumerdCount *atomic.Int32
}

var (
	bufferSize = 1024 * 1024
)

func dial(
	ctx context.Context,
	dialer *bufconn.Listener,
) *grpc.ClientConn {
	// the uuid here is not relevant, just needs to be unique (pretty sure)
	cc, err := servers.Dial(ctx, uuid.NewString(), servers.WithDialOpts(
		grpc.WithContextDialer(
			func(context.Context, string) (net.Conn, error) {
				return dialer.Dial()
			}),
	))
	if err != nil {
		panic(err)
	}
	return cc
}

func (e *Environment) SpawnAgent() (context.Context, context.CancelFunc) {
	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName(string(rune('a'+e.agentCount.Load()))),
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
			ConcurrentProcessLimit:  int32(e.Config.Agent.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: e.Config.Agent.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   e.Config.Agent.UsageLimits.QueueRejectMultiplier,
		}),
		agent.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		agent.WithToolchainRunners(testtoolchain.AddToStore),
	}
	if addr := e.Config.Monitor.ListenAddress; addr != "" {
		cc := dial(ctx, e.portMapping[addr])
		options = append(options,
			agent.WithMonitorClient(types.NewMonitorClient(cc)))
	}

	if addr := e.Config.Scheduler.ListenAddress; addr != "" {
		cc := dial(ctx, e.portMapping[addr])
		options = append(options,
			agent.WithSchedulerClient(types.NewSchedulerClient(cc)))
	}

	agentSrv := agent.NewAgentServer(ctx, options...)
	mgr := servers.NewStreamManager(ctx, agentSrv)
	go mgr.Run()
	go agentSrv.StartMetricsProvider()
	return ctx, cancel
}

func (e *Environment) SpawnScheduler() (context.Context, context.CancelFunc) {
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
	lg := meta.Log(ctx)

	srv := servers.NewServer(ctx)

	options := []scheduler.SchedulerServerOption{}
	if addr := e.Config.Monitor.ListenAddress; addr != "" {
		cc := dial(ctx, e.portMapping[addr])
		options = append(options,
			scheduler.WithMonitorClient(types.NewMonitorClient(cc)))
	}

	if addr := e.Config.Cache.ListenAddress; addr != "" {
		cc := dial(ctx, e.portMapping[addr])
		options = append(options,
			scheduler.WithCacheClient(types.NewCacheClient(cc)))
	}

	sc := scheduler.NewSchedulerServer(ctx, options...)
	types.RegisterSchedulerServer(srv, sc)
	go sc.StartMetricsProvider()
	go func() {
		if err := srv.Serve(e.portMapping[e.Config.Scheduler.ListenAddress]); err != nil {
			lg.Info(err)
		}
	}()
	return ctx, cancel
}

func (e *Environment) SpawnConsumerd(addr string) (context.Context, context.CancelFunc) {
	ctx := meta.NewContextWithParent(e.envContext,
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Consumerd,
				logkc.WithName(string(rune('a'+e.consumerdCount.Load()))),
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

	lg := meta.Log(ctx)

	listener := bufconn.Listen(bufferSize)
	e.portMapping[addr] = listener

	options := []consumerd.ConsumerdServerOption{
		consumerd.WithUsageLimits(&metrics.UsageLimits{
			ConcurrentProcessLimit:  int32(e.Config.Consumerd.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: e.Config.Consumerd.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   e.Config.Consumerd.UsageLimits.QueueRejectMultiplier,
		}),
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		consumerd.WithToolchainRunners(testtoolchain.AddToStore),
	}
	if addr := e.Config.Monitor.ListenAddress; addr != "" {
		cc := dial(ctx, e.portMapping[addr])
		options = append(options,
			consumerd.WithMonitorClient(types.NewMonitorClient(cc)))
	}

	if addr := e.Config.Scheduler.ListenAddress; addr != "" {
		cc := dial(ctx, e.portMapping[addr])
		options = append(options,
			consumerd.WithSchedulerClient(types.NewSchedulerClient(cc)))
	}
	srv := servers.NewServer(ctx)
	cd := consumerd.NewConsumerdServer(ctx, options...)
	types.RegisterConsumerdServer(srv, cd)

	go cd.StartMetricsProvider()
	go func() {
		if err := srv.Serve(listener); err != nil {
			lg.Info(err)
		}
	}()

	return ctx, cancel
}

func (e *Environment) SpawnMonitor() (context.Context, context.CancelFunc) {
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
	lg := meta.Log(ctx)

	srv := servers.NewServer(ctx)

	mon := monitor.NewMonitorServer(ctx, monitor.InMemoryStoreCreator)
	types.RegisterMonitorServer(srv, mon)

	go func() {
		if err := srv.Serve(e.portMapping[e.Config.Monitor.ListenAddress]); err != nil {
			lg.Info(err)
		}
	}()
	return ctx, cancel
}

func (e *Environment) SpawnCache() (context.Context, context.CancelFunc) {
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
	lg := meta.Log(ctx)

	options := []cachesrv.CacheServerOption{}
	if addr := e.Config.Monitor.ListenAddress; addr != "" {
		cc := dial(ctx, e.portMapping[addr])
		options = append(options,
			cachesrv.WithMonitorClient(types.NewMonitorClient(cc)))
	}

	providers := []storage.StorageProvider{}
	if e.Config.Cache.LocalStorage != nil {
		providers = append(providers,
			storage.NewVolatileStorageProvider(ctx, *e.Config.Cache.LocalStorage))
	}
	if e.Config.Cache.RemoteStorage != nil {
		providers = append(providers,
			storage.NewS3StorageProvider(ctx, *e.Config.Cache.RemoteStorage))
	}
	options = append(options, cachesrv.WithStorageProvider(
		storage.NewChainStorageProvider(ctx, providers...),
	))

	cacheSrv := cachesrv.NewCacheServer(ctx, e.Config.Cache, options...)
	srv := servers.NewServer(ctx)

	types.RegisterCacheServer(srv, cacheSrv)
	go cacheSrv.StartMetricsProvider()

	go func() {
		err := srv.Serve(e.portMapping[e.Config.Cache.ListenAddress])
		if err != nil {
			lg.With(zap.Error(err)).Error("GRPC error")
		}
	}()
	return ctx, cancel
}

func NewDefaultEnvironment() *Environment {
	schedulerAddr := "9000"
	monitorAddr := "9001"
	cacheAddr := "9002"

	return &Environment{
		Config: &config.KubeccSpec{
			Global: config.GlobalSpec{
				LogLevel: "debug",
			},
			Agent: config.AgentSpec{
				UsageLimits: config.UsageLimitsSpec{
					ConcurrentProcessLimit:  32,
					QueuePressureMultiplier: 1.0,
					QueueRejectMultiplier:   2.0,
				},
				SchedulerAddress: schedulerAddr,
				MonitorAddress:   monitorAddr,
			},
			Scheduler: config.SchedulerSpec{
				MonitorAddress: monitorAddr,
				CacheAddress:   cacheAddr,
				ListenAddress:  schedulerAddr,
			},
			Monitor: config.MonitorSpec{
				ListenAddress: monitorAddr,
			},
			Cache: config.CacheSpec{
				ListenAddress:  cacheAddr,
				MonitorAddress: monitorAddr,
				LocalStorage: &config.LocalStorageSpec{
					Limits: config.StorageLimitsSpec{
						Memory: "1Gi",
					},
				},
			},
			Consumerd: config.ConsumerdSpec{
				SchedulerAddress: schedulerAddr,
				MonitorAddress:   monitorAddr,
				DisableTLS:       true,
				UsageLimits: config.UsageLimitsSpec{
					ConcurrentProcessLimit: 20,
				},
			},
		},
	}
}

func (e *Environment) Start() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.TestComponent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	ctx, cancel := context.WithCancel(ctx)
	e.envContext = ctx
	e.envCancel = cancel

	e.agentCount = atomic.NewInt32(0)
	e.consumerdCount = atomic.NewInt32(0)
	e.portMapping = make(map[string]*bufconn.Listener)

	for _, addr := range []string{
		e.Config.Cache.ListenAddress,
		e.Config.Monitor.ListenAddress,
		e.Config.Scheduler.ListenAddress,
	} {
		if addr != "" {
			e.portMapping[addr] = bufconn.Listen(bufferSize)
		}
	}
	tracer, _ := tracing.Start(e.envContext, types.TestComponent)
	opentracing.SetGlobalTracer(tracer)
}

func (e *Environment) WaitForServers(count int) {
	wg := sync.WaitGroup{}
	wg.Add(count)
	monClient := e.NewMonitorClient()
	listener := clients.NewListener(e.envContext, monClient)
	listener.OnProviderAdded(func(pctx context.Context, uuid string) {
		wg.Done()
	})
	wg.Wait()
}

func (e *Environment) Dial(addr string) *grpc.ClientConn {
	return dial(e.envContext, e.portMapping[addr])
}

func (e *Environment) NewMonitorClient() types.MonitorClient {
	return types.NewMonitorClient(
		dial(e.envContext, e.portMapping[e.Config.Monitor.ListenAddress]))
}

func (e *Environment) NewSchedulerClient() types.SchedulerClient {
	return types.NewSchedulerClient(
		dial(e.envContext, e.portMapping[e.Config.Scheduler.ListenAddress]))
}

func (e *Environment) NewCacheClient() types.CacheClient {
	return types.NewCacheClient(
		dial(e.envContext, e.portMapping[e.Config.Cache.ListenAddress]))
}

func (e *Environment) Shutdown() {
	e.envCancel()
}
