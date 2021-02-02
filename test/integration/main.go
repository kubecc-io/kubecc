package main

import (
	"context"
	"net"

	"github.com/cobalt77/kubecc/internal/lll"
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

var (
	agent1Listener = bufconn.Listen(bufSize)
	schedListener  = bufconn.Listen(bufSize)
	cdListener     = bufconn.Listen(bufSize)
)

func dial(
	ctx context.Context,
	dialer *bufconn.Listener,
) (context.Context, *grpc.ClientConn) {
	cc, err := servers.Dial(ctx, "bufnet", servers.With(
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

func runAgent() {
	ctx := lll.NewFromContext(cluster.NewAgentContext(), lll.Agent)
	srv := servers.NewServer(ctx)

	agent := agent.NewAgentServer(ctx)
	types.RegisterAgentServer(srv, agent)
	go func() {
		ctx, cc := dial(ctx, schedListener)
		schedClient := types.NewSchedulerClient(cc)
		schedClient.Connect(ctx)
	}()
	srv.Serve(agent1Listener)
}

func runScheduler() {
	ctx := lll.NewFromContext(context.Background(), lll.Scheduler)

	srv := servers.NewServer(ctx)

	sc := scheduler.NewSchedulerServer(ctx)
	types.RegisterSchedulerServer(srv, sc)
	srv.Serve(schedListener)
}

func runConsumerd() {
	ctx := lll.NewFromContext(context.Background(), lll.Consumerd)

	srv := servers.NewServer(ctx)

	d := consumerd.NewConsumerdServer(ctx)
	types.RegisterConsumerdServer(srv, d)
	go func() {
		ctx, cc := dial(ctx, schedListener)
		/* schedClient := */ types.NewSchedulerClient(cc)

		<-ctx.Done()
	}()
	srv.Serve(cdListener)
}

func runConsumer() {
	ctx := lll.NewFromContext(context.Background(), lll.Consumer)

	ctx, cc := dial(ctx, cdListener)
	/*cdClient := */ types.NewConsumerdClient(cc)
	<-ctx.Done()
}
func main() {
	viper.Set("scheduler", "roundRobinDns")
	viper.Set("remoteOnly", "false")
	viper.Set("arch", "amd64")
	viper.Set("cpus", 4)
	viper.Set("node", "test-node")
	viper.Set("pod", "test-pod")
	viper.Set("namespace", "test-namespace")

	go runAgent()
	go runScheduler()
	go runConsumerd()
	go runConsumer()

	<-ctrl.SetupSignalHandler().Done()
}
