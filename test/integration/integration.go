package integration

import (
	"context"
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	agent "github.com/cobalt77/kubecc/pkg/apps/agent"
	consumerd "github.com/cobalt77/kubecc/pkg/apps/consumerd"
	scheduler "github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	ctrl "sigs.k8s.io/controller-runtime"
)

const bufSize = 1024 * 1024

type TestController struct {
	Consumers []types.ConsumerdClient

	agentListeners []*bufconn.Listener
	cdListeners    []*bufconn.Listener
	schedListener  *bufconn.Listener
}

func dial(
	ctx context.Context,
	dialer *bufconn.Listener,
) (context.Context, *grpc.ClientConn) {
	cc, err := servers.Dial(ctx, "", servers.With(
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
	srv := servers.NewServer(ctx)

	listener := bufconn.Listen(bufSize)
	tc.agentListeners = append(tc.agentListeners, listener)
	agent := agent.NewAgentServer(ctx)
	types.RegisterAgentServer(srv, agent)
	go func() {
		ctx, cc := dial(ctx, tc.schedListener)
		client := types.NewSchedulerClient(cc)
		c, err := client.Connect(ctx)
		if err != nil {
			panic(err)
		}
		c.Send(&types.Metadata{
			Component: types.Agent,
			Toolchains: []*types.Toolchain{
				{
					Kind:       types.Gnu,
					Lang:       types.CXX,
					Executable: "g++",
					TargetArch: "x86_64",
					Version:    "10",
					PicDefault: true,
				},
				{
					Kind:       types.Gnu,
					Lang:       types.C,
					Executable: "gcc",
					TargetArch: "x86_64",
					Version:    "10",
					PicDefault: true,
				},
			},
		})
		select {
		case <-ctx.Done():
		case <-c.Context().Done():
		}
	}()
	go srv.Serve(listener)
}

func (tc *TestController) runScheduler() {
	ctx := logkc.NewFromContext(context.Background(), types.Scheduler,
		logkc.WithName("a"),
	)
	tc.schedListener = bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)

	sc := scheduler.NewSchedulerServer(ctx)
	types.RegisterSchedulerServer(srv, sc)
	go srv.Serve(tc.schedListener)
}

func (tc *TestController) runConsumerd() {
	ctx := logkc.NewFromContext(context.Background(), types.Consumerd,
		logkc.WithName(string(rune('a'+len(tc.cdListeners)))),
	)
	listener := bufconn.Listen(bufSize)
	srv := servers.NewServer(ctx)

	d := consumerd.NewConsumerdServer(ctx)
	types.RegisterConsumerdServer(srv, d)

	go func() {
		ctx, cc := dial(ctx, tc.schedListener)
		client := types.NewSchedulerClient(cc)
		c, err := client.Connect(ctx)
		if err != nil {
			panic(err)
		}
		c.Send(&types.Metadata{
			Component: types.Consumerd,
			Toolchains: []*types.Toolchain{
				{
					Kind:       types.Gnu,
					Lang:       types.CXX,
					Executable: "g++",
					TargetArch: "x86_64",
					Version:    "10",
					PicDefault: true,
				},
				{
					Kind:       types.Gnu,
					Lang:       types.C,
					Executable: "gcc",
					TargetArch: "x86_64",
					Version:    "10",
					PicDefault: true,
				},
			},
		})
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
	viper.Set("cpus", 4)
	viper.Set("node", "test-node")
	viper.Set("pod", "test-pod")
	viper.Set("namespace", "test-namespace")

	tc.runScheduler()
	for i := 0; i < ops.NumAgents; i++ {
		tc.runAgent()
	}
	for i := 0; i < ops.NumClients; i++ {
		tc.runConsumerd()
	}
}

func (tc *TestController) Wait() {
	<-ctrl.SetupSignalHandler().Done()
}
