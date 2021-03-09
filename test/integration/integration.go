package integration

import (
	"context"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil/toolchain"
	agent "github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/apps/cachesrv"
	consumerd "github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	scheduler "github.com/cobalt77/kubecc/pkg/apps/scheduler"
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
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	ctrl "sigs.k8s.io/controller-runtime"
)

const bufSize = 1024 * 1024

type TestController struct {
	Consumers          []types.ConsumerdClient
	ctx                context.Context
	cancel             context.CancelFunc
	agentListeners     map[string]*bufconn.Listener
	agentListenersLock *sync.Mutex
	schedListener      *bufconn.Listener
	monListener        *bufconn.Listener
	cacheListener      *bufconn.Listener
}

func NewTestController(ctx context.Context) *TestController {
	ctx, cancel := context.WithCancel(ctx)
	return &TestController{
		ctx:                ctx,
		cancel:             cancel,
		agentListeners:     make(map[string]*bufconn.Listener),
		agentListenersLock: &sync.Mutex{},
		Consumers:          []types.ConsumerdClient{},
	}
}

func (tc *TestController) Dial(ctx context.Context) (types.AgentClient, error) {
	tc.agentListenersLock.Lock()
	defer tc.agentListenersLock.Unlock()

	listener := tc.agentListeners[meta.UUID(ctx)]
	cc := dial(ctx, listener)
	return types.NewAgentClient(cc), nil
}

func dial(
	ctx context.Context,
	dialer *bufconn.Listener,
) *grpc.ClientConn {
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

func (tc *TestController) startAgent(cfg *types.UsageLimits) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName(string(rune('a'+len(tc.agentListeners)))),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)
	srv := servers.NewServer(ctx)

	listener := bufconn.Listen(bufSize)
	tc.agentListeners[meta.UUID(ctx)] = listener
	cc := dial(ctx, tc.schedListener)
	schedClient := types.NewSchedulerClient(cc)
	cc = dial(ctx, tc.monListener)
	internalMonClient := types.NewInternalMonitorClient(cc)
	agentSrv := agent.NewAgentServer(ctx,
		agent.WithSchedulerClient(schedClient),
		agent.WithMonitorClient(internalMonClient),
		agent.WithUsageLimits(cfg),
		agent.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		agent.WithToolchainRunners(testtoolchain.AddToStore),
	)
	types.RegisterAgentServer(srv, agentSrv)
	mgr := servers.NewStreamManager(ctx, agentSrv)
	go mgr.Run()
	go agentSrv.StartMetricsProvider()
	go func() {
		if err := srv.Serve(listener); err != nil {
			lg.Info(err)
		}
	}()
}

func (tc *TestController) startScheduler() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Scheduler,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	tc.schedListener = bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)

	cc := dial(ctx, tc.monListener)
	internalMonClient := types.NewInternalMonitorClient(cc)

	cc = dial(ctx, tc.cacheListener)
	cacheClient := types.NewCacheClient(cc)

	sc := scheduler.NewSchedulerServer(ctx,
		scheduler.WithSchedulerOptions(
			scheduler.WithAgentDialer(tc),
		),
		scheduler.WithMonitorClient(internalMonClient),
		scheduler.WithCacheClient(cacheClient),
	)
	types.RegisterSchedulerServer(srv, sc)
	go sc.StartMetricsProvider()
	go func() {
		if err := srv.Serve(tc.schedListener); err != nil {
			lg.Info(err)
		}
	}()
}

func (tc *TestController) startMonitor() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Monitor,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	tc.monListener = bufconn.Listen(bufSize)
	internalSrv := servers.NewServer(ctx)
	externalSrv := servers.NewServer(ctx)
	extListener, err := net.Listen("tcp", "127.0.0.1:9097")
	if err != nil {
		panic(err)
	}

	mon := monitor.NewMonitorServer(ctx, monitor.InMemoryStoreCreator)
	types.RegisterInternalMonitorServer(internalSrv, mon)
	types.RegisterExternalMonitorServer(externalSrv, mon)

	go func() {
		if err := internalSrv.Serve(tc.monListener); err != nil {
			lg.Info(err)
		}
	}()

	go func() {
		if err := externalSrv.Serve(extListener); err != nil {
			lg.Info(err)
		}
	}()
}

func (tc *TestController) startCache() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Cache)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Cache,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	cc := dial(ctx, tc.monListener)
	internalMonClient := types.NewInternalMonitorClient(cc)

	tc.cacheListener = bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)
	cache := cachesrv.NewCacheServer(ctx, config.CacheSpec{},
		cachesrv.WithStorageProvider(
			storage.NewChainStorageProvider(ctx,
				storage.NewVolatileStorageProvider(ctx,
					config.LocalStorageSpec{
						Limits: config.StorageLimitsSpec{
							Memory: "4Gi",
						},
					},
				),
				storage.NewS3StorageProvider(ctx,
					config.RemoteStorageSpec{
						Endpoint:  "192.168.0.84:9000",
						AccessKey: "minioadmin",
						SecretKey: "minioadmin",
						TLS:       false,
						Bucket:    "kubecc",
					},
				),
			),
		),
		cachesrv.WithMonitorClient(internalMonClient),
	)
	types.RegisterCacheServer(srv, cache)
	go cache.StartMetricsProvider()

	go func() {
		err := srv.Serve(tc.cacheListener)
		if err != nil {
			lg.With(zap.Error(err)).Error("GRPC error")
		}
	}()
}

func (tc *TestController) startConsumerd(cfg *types.UsageLimits) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Consumerd,
				logkc.WithName(string(rune('a'+len(tc.Consumers)))),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)

	listener := bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)
	cc := dial(ctx, tc.schedListener)
	schedulerClient := types.NewSchedulerClient(cc)
	cc = dial(ctx, tc.monListener)
	monitorClient := types.NewInternalMonitorClient(cc)

	d := consumerd.NewConsumerdServer(ctx,
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		consumerd.WithUsageLimits(cfg),
		consumerd.WithToolchainRunners(testtoolchain.AddToStore),
		consumerd.WithToolchainRunners(testtoolchain.AddToStore),
		consumerd.WithSchedulerClient(schedulerClient, cc),
		consumerd.WithMonitorClient(monitorClient),
	)
	types.RegisterConsumerdServer(srv, d)

	mgr := servers.NewStreamManager(ctx, d)
	go mgr.Run()
	go d.StartMetricsProvider()
	cdListener := dial(ctx, listener)
	cdClient := types.NewConsumerdClient(cdListener)
	tc.Consumers = append(tc.Consumers, cdClient)
	go func() {
		if err := srv.Serve(listener); err != nil {
			lg.Info(err)
		}
	}()
}

type TestOptions struct {
	Clients []*types.UsageLimits
	Agents  []*types.UsageLimits
}

func (tc *TestController) Start(ops TestOptions) {
	tc.agentListenersLock.Lock()
	defer tc.agentListenersLock.Unlock()

	tracer, _ := tracing.Start(tc.ctx, types.TestComponent)
	opentracing.SetGlobalTracer(tracer)

	tc.startMonitor()
	tc.startCache()
	tc.startScheduler()
	for _, cfg := range ops.Agents {
		tc.startAgent(cfg)
	}
	for _, cfg := range ops.Clients {
		tc.startConsumerd(cfg)
	}

	// Hook into the metrics server and wait for all the components to load up
	cc, err := servers.Dial(tc.ctx, "127.0.0.1:9097")
	if err != nil {
		panic(err)
	}
	waitCtx, waitCancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(
		len(ops.Agents) +
			len(ops.Agents) +
			1 /*scheduler*/ +
			1 /*cache*/)
	extClient := types.NewExternalMonitorClient(cc)
	listener := metrics.NewListener(tc.ctx, extClient)
	// todo this doesnt get all the providers
	listener.OnProviderAdded(func(pctx context.Context, uuid string) {
		resp, _ := extClient.Whois(tc.ctx, &types.WhoisRequest{
			UUID: uuid,
		})
		if resp.Component == types.Monitor {
			return
		}
		wg.Done()
		select {
		case <-pctx.Done():
		case <-waitCtx.Done():
		}
	})
	wg.Wait()
	waitCancel()
}

func (tc *TestController) Teardown() {
	tc.cancel()
}

func (tc *TestController) Wait() {
	<-ctrl.SetupSignalHandler().Done()
}
