package integration

import (
	"context"
	"fmt"
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil/toolchain"
	agent "github.com/cobalt77/kubecc/pkg/apps/agent"
	consumerd "github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	scheduler "github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/cluster"
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
	Consumers      []types.ConsumerdClient
	ctx            context.Context
	cancel         context.CancelFunc
	agentListeners map[types.AgentID]*bufconn.Listener
	schedListener  *bufconn.Listener
	monListener    *bufconn.Listener
}

func NewTestController(ctx context.Context) *TestController {
	ctx, cancel := context.WithCancel(ctx)
	return &TestController{
		ctx:            ctx,
		cancel:         cancel,
		agentListeners: make(map[types.AgentID]*bufconn.Listener),
		Consumers:      []types.ConsumerdClient{},
	}
}

func (tc *TestController) Dial(ctx context.Context) (types.AgentClient, error) {
	info, _ := cluster.AgentInfoFromContext(ctx)
	id, _ := info.AgentID()
	listener := tc.agentListeners[id]
	_, cc := dial(ctx, listener)
	return types.NewAgentClient(cc), nil
}

func dial(
	ctx context.Context,
	dialer *bufconn.Listener,
) (context.Context, *grpc.ClientConn) {
	cc, err := servers.Dial(ctx, uuid.NewString(), servers.With(
		grpc.WithContextDialer(
			func(context.Context, string) (net.Conn, error) {
				return dialer.Dial()
			}),
	))
	if err != nil {
		panic(err)
	}
	return ctx, cc
}

func (tc *TestController) runAgent(cfg *types.CpuConfig) {
	ctx := logkc.NewWithContext(
		cluster.ContextWithAgentInfo(
			tc.ctx, cluster.MakeAgentInfo()), types.Agent,
		logkc.WithName(string(rune('a'+len(tc.agentListeners)))),
	)
	lg := logkc.LogFromContext(ctx)
	info := cluster.MakeAgentInfo()
	ctx = cluster.ContextWithAgentInfo(ctx, info)
	tracer, closer := tracing.Start(ctx, types.Agent)
	ctx = tracing.ContextWithTracer(ctx, tracer)
	srv := servers.NewServer(ctx)

	listener := bufconn.Listen(bufSize)
	id, _ := info.AgentID()
	tc.agentListeners[id] = listener
	ctx, cc := dial(ctx, tc.schedListener)
	schedClient := types.NewSchedulerClient(cc)
	ctx, cc = dial(ctx, tc.monListener)
	internalMonClient := types.NewInternalMonitorClient(cc)
	agentSrv := agent.NewAgentServer(ctx,
		agent.WithSchedulerClient(schedClient),
		agent.WithMonitorClient(internalMonClient),
		agent.WithCpuConfig(cfg),
		agent.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		agent.WithToolchainRunners(testtoolchain.AddToStore),
	)
	types.RegisterAgentServer(srv, agentSrv)

	go agentSrv.RunSchedulerClient()
	go agentSrv.StartMetricsProvider()
	go func() {
		defer closer.Close()
		if err := srv.Serve(listener); err != nil {
			lg.Info(err)
		}
	}()
}

func (tc *TestController) runScheduler() {
	ctx := logkc.NewWithContext(tc.ctx, types.Scheduler,
		logkc.WithName("a"),
	)
	lg := logkc.LogFromContext(ctx)
	tracer, closer := tracing.Start(ctx, types.Scheduler)
	ctx = tracing.ContextWithTracer(ctx, tracer)
	tc.schedListener = bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)

	sc := scheduler.NewSchedulerServer(ctx, scheduler.WithAgentDialer(tc))
	types.RegisterSchedulerServer(srv, sc)
	go func() {
		defer closer.Close()
		if err := srv.Serve(tc.schedListener); err != nil {
			lg.Info(err)
		}
	}()
}

func (tc *TestController) runMonitor() {
	ctx := logkc.NewWithContext(tc.ctx, types.Monitor,
		logkc.WithName("a"),
	)
	lg := logkc.LogFromContext(ctx)
	tracer, closer := tracing.Start(ctx, types.Monitor)
	ctx = tracing.ContextWithTracer(ctx, tracer)
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
		defer closer.Close()
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

func (tc *TestController) runConsumerd(cfg *types.CpuConfig) {
	ctx := logkc.NewWithContext(tc.ctx, types.Consumerd,
		logkc.WithName(string(rune('a'+len(tc.Consumers)))),
	)
	lg := logkc.LogFromContext(ctx)
	tracer, closer := tracing.Start(ctx, types.Consumerd)
	ctx = tracing.ContextWithTracer(ctx, tracer)
	listener := bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)
	ctx, cc := dial(ctx, tc.schedListener)
	client := types.NewSchedulerClient(cc)

	d := consumerd.NewConsumerdServer(ctx,
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		consumerd.WithCpuConfig(cfg),
		consumerd.WithToolchainRunners(testtoolchain.AddToStore),
		consumerd.WithSchedulerClient(client, cc),
	)
	types.RegisterConsumerdServer(srv, d)

	ready := make(chan struct{})

	go func() {
		c, err := client.ConnectConsumerd(ctx, grpc.WaitForReady(true))
		if err != nil {
			panic(err)
		}
		close(ready)
		select {
		case <-ctx.Done():
		case <-c.Context().Done():
		}
	}()

	<-ready
	lg.Info(types.Consumerd.Color().Add("Connected to the scheduler"))

	_, cdListener := dial(ctx, listener)
	cdClient := types.NewConsumerdClient(cdListener)
	tc.Consumers = append(tc.Consumers, cdClient)
	go func() {
		defer closer.Close()
		if err := srv.Serve(listener); err != nil {
			lg.Info(err)
		}
	}()
}

type TestOptions struct {
	Clients []*types.CpuConfig
	Agents  []*types.CpuConfig
}

func (tc *TestController) Start(ops TestOptions) {
	viper.Set("remoteOnly", "false")
	viper.Set("arch", "amd64")
	viper.Set("namespace", "test-namespace")

	tracer, _ := tracing.Start(tc.ctx, types.TestComponent)
	opentracing.SetGlobalTracer(tracer)

	tc.runScheduler()
	tc.runMonitor()
	for i, cfg := range ops.Agents {
		viper.Set("node", fmt.Sprintf("test-node-%d", i))
		viper.Set("pod", fmt.Sprintf("test-pod-%d", i))
		tc.runAgent(cfg)
	}
	for _, cfg := range ops.Clients {
		tc.runConsumerd(cfg)
	}
}

func (tc *TestController) Teardown() {
	tc.schedListener.Close()
	for _, v := range tc.agentListeners {
		v.Close()
	}
	tc.cancel()
}

func (tc *TestController) Wait() {
	<-ctrl.SetupSignalHandler().Done()
}
