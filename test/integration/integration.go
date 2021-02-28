package integration

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil/toolchain"
	agent "github.com/cobalt77/kubecc/pkg/apps/agent"
	consumerd "github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	scheduler "github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	ctrl "sigs.k8s.io/controller-runtime"
)

const bufSize = 1024 * 1024

type TestController struct {
	Consumers          []types.ConsumerdClient
	ctx                meta.Context
	cancel             context.CancelFunc
	agentListeners     map[string]*bufconn.Listener
	agentListenersLock *sync.Mutex

	schedListener *bufconn.Listener
	monListener   *bufconn.Listener
}

func NewTestController(ctx meta.Context) *TestController {
	_, cancel := context.WithCancel(ctx)
	return &TestController{
		ctx:                ctx,
		cancel:             cancel,
		agentListeners:     make(map[string]*bufconn.Listener),
		agentListenersLock: &sync.Mutex{},
		Consumers:          []types.ConsumerdClient{},
	}
}

func (tc *TestController) Dial(ctx meta.Context) (types.AgentClient, error) {
	tc.agentListenersLock.Lock()
	defer tc.agentListenersLock.Unlock()

	listener := tc.agentListeners[ctx.UUID()]
	cc := dial(ctx, listener)
	return types.NewAgentClient(cc), nil
}

func dial(
	ctx context.Context,
	dialer *bufconn.Listener,
) *grpc.ClientConn {
	cc, err := servers.Dial(ctx, uuid.NewString(), servers.With(
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
		meta.WithProvider(logkc.MetadataProvider, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName(string(rune('a'+len(tc.agentListeners)))),
			),
		)),
		meta.WithProvider(tracing.MetadataProvider),
	)
	lg := ctx.Log()
	srv := servers.NewServer(ctx)

	listener := bufconn.Listen(bufSize)
	tc.agentListeners[ctx.UUID()] = listener
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

	go agentSrv.RunSchedulerClient()
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
		meta.WithProvider(logkc.MetadataProvider, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.MetadataProvider),
	)
	lg := ctx.Log()

	tc.schedListener = bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)

	sc := scheduler.NewSchedulerServer(ctx, scheduler.WithAgentDialer(tc))
	types.RegisterSchedulerServer(srv, sc)
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
		meta.WithProvider(logkc.MetadataProvider, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName("a"),
			),
		)),
		meta.WithProvider(tracing.MetadataProvider),
	)
	lg := ctx.Log()

	tc.monListener = bufconn.Listen(bufSize)
	internalSrv := servers.NewServer(ctx)
	externalSrv := servers.NewServer(ctx)
	extListener, err := net.Listen("tcp", "127.0.0.1:9960")
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

func (tc *TestController) startConsumerd(cfg *types.UsageLimits) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.MetadataProvider, meta.WithValue(
			logkc.New(types.Agent,
				logkc.WithName(string(rune('a'+len(tc.Consumers)))),
			),
		)),
		meta.WithProvider(tracing.MetadataProvider),
	)
	lg := ctx.Log()

	listener := bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)
	cc := dial(ctx, tc.schedListener)
	client := types.NewSchedulerClient(cc)

	d := consumerd.NewConsumerdServer(ctx,
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		consumerd.WithUsageLimits(cfg),
		consumerd.WithToolchainRunners(testtoolchain.AddToStore),
		consumerd.WithSchedulerClient(client, cc),
	)
	types.RegisterConsumerdServer(srv, d)

	go d.RunSchedulerClient()

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

	viper.Set("remoteOnly", "false")
	viper.Set("arch", "amd64")
	viper.Set("namespace", "test-namespace")

	tracer, _ := tracing.Start(tc.ctx, types.TestComponent)
	opentracing.SetGlobalTracer(tracer)

	tc.startScheduler()
	tc.startMonitor()
	for i, cfg := range ops.Agents {
		viper.Set("node", fmt.Sprintf("test-node-%d", i))
		viper.Set("pod", fmt.Sprintf("test-pod-%d", i))
		tc.startAgent(cfg)
	}
	for _, cfg := range ops.Clients {
		tc.startConsumerd(cfg)
	}
}

func (tc *TestController) Teardown() {
	tc.schedListener.Close()
	tc.agentListenersLock.Lock()
	for _, v := range tc.agentListeners {
		v.Close()
	}
	tc.agentListenersLock.Unlock()
	tc.cancel()
}

func (tc *TestController) Wait() {
	<-ctrl.SetupSignalHandler().Done()
}
