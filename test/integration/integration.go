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
	scheduler "github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	ctrl "sigs.k8s.io/controller-runtime"
)

const bufSize = 1024 * 1024

type TestController struct {
	Consumers []types.ConsumerdClient

	agentListeners map[types.AgentID]*bufconn.Listener
	cdListeners    []*bufconn.Listener
	schedListener  *bufconn.Listener
}

func NewTestController() *TestController {
	return &TestController{
		agentListeners: make(map[types.AgentID]*bufconn.Listener),
		cdListeners:    []*bufconn.Listener{},
		Consumers:      []types.ConsumerdClient{},
	}
}

func (tc *TestController) Dial(ctx context.Context) (types.AgentClient, error) {
	info, _ := cluster.AgentInfoFromContext(ctx)
	id, _ := info.AgentID()
	listener := tc.agentListeners[id]
	_, cc := dial(context.Background(), listener)
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

func (tc *TestController) runAgent() {
	ctx := logkc.NewFromContext(cluster.NewAgentContext(), types.Agent,
		logkc.WithName(string(rune('a'+len(tc.agentListeners)))),
	)
	info := cluster.MakeAgentInfo()
	ctx = cluster.ContextWithAgentInfo(ctx, info)
	srv := servers.NewServer(ctx)

	listener := bufconn.Listen(bufSize)
	id, _ := info.AgentID()
	tc.agentListeners[id] = listener
	ctx, cc := dial(ctx, tc.schedListener)
	client := types.NewSchedulerClient(cc)
	agentSrv := agent.NewAgentServer(ctx,
		agent.WithSchedulerClient(client),
		agent.WithCpuConfig(&types.CpuConfig{
			MaxRunningProcesses:    4,
			QueuePressureThreshold: 1.0,
			QueueRejectThreshold:   2.0,
		}),
		agent.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		agent.WithToolchainRunners(testtoolchain.AddToStore),
	)
	types.RegisterAgentServer(srv, agentSrv)

	go agentSrv.RunSchedulerClient(ctx)
	go srv.Serve(listener)
}

func (tc *TestController) runScheduler() {
	ctx := logkc.NewFromContext(context.Background(), types.Scheduler,
		logkc.WithName("a"),
	)
	tc.schedListener = bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)

	sc := scheduler.NewSchedulerServer(ctx, scheduler.WithAgentDialer(tc))
	types.RegisterSchedulerServer(srv, sc)
	go srv.Serve(tc.schedListener)
}

func (tc *TestController) runConsumerd() {
	ctx := logkc.NewFromContext(context.Background(), types.Consumerd,
		logkc.WithName(string(rune('a'+len(tc.cdListeners)))),
	)
	listener := bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)

	d := consumerd.NewConsumerdServer(ctx,
		consumerd.WithToolchainFinders(toolchains.FinderWithOptions{
			Finder: testutil.TestToolchainFinder{},
		}),
		consumerd.WithToolchainRunners(testtoolchain.AddToStore),
	)
	types.RegisterConsumerdServer(srv, d)

	go func() {
		ctx, cc := dial(ctx, tc.schedListener)
		client := types.NewSchedulerClient(cc)
		c, err := client.ConnectConsumerd(ctx)
		if err != nil {
			panic(err)
		}
		select {
		case <-ctx.Done():
		case <-c.Context().Done():
		}
	}()

	_, cc := dial(ctx, listener)
	client := types.NewConsumerdClient(cc)
	tc.Consumers = append(tc.Consumers, client)

	go srv.Serve(listener)
}

type TestOptions struct {
	NumClients int
	NumAgents  int
}

func (tc *TestController) Start(ops TestOptions) {
	viper.Set("remoteOnly", "false")
	viper.Set("arch", "amd64")
	viper.Set("namespace", "test-namespace")

	tc.runScheduler()
	for i := 0; i < ops.NumAgents; i++ {
		viper.Set("node", fmt.Sprintf("test-node-%d", i))
		viper.Set("pod", fmt.Sprintf("test-pod-%d", i))
		tc.runAgent()
	}
	for i := 0; i < ops.NumClients; i++ {
		tc.runConsumerd()
	}
}

func (tc *TestController) Wait() {
	<-ctrl.SetupSignalHandler().Done()
}
