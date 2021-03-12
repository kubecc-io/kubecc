package commands

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/sleep"
	sleeptoolchain "github.com/cobalt77/kubecc/internal/sleep/toolchain"
	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/cc"
	cctoolchain "github.com/cobalt77/kubecc/pkg/cc/toolchain"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	_ "google.golang.org/grpc/encoding/gzip"
)

func run(cmd *cobra.Command, args []string) {
	conf := (&config.ConfigMapProvider{}).Load().Agent
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Agent)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.Agent,
				logkc.WithLogLevel(conf.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatalw("Error listening on socket")
	}

	schedulerCC, err := servers.Dial(ctx, conf.SchedulerAddress)
	lg.With("address", schedulerCC.Target()).Info("Dialing scheduler")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing scheduler")
	}

	monitorCC, err := servers.Dial(ctx, conf.MonitorAddress)
	lg.With("address", monitorCC.Target()).Info("Dialing monitor")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing monitor")
	}

	schedulerClient := types.NewSchedulerClient(schedulerCC)
	monitorClient := types.NewInternalMonitorClient(monitorCC)

	a := agent.NewAgentServer(ctx,
		agent.WithUsageLimits(&types.UsageLimits{
			ConcurrentProcessLimit:  int32(conf.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: conf.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   conf.UsageLimits.QueueRejectMultiplier,
		}),
		agent.WithToolchainFinders(
			toolchains.FinderWithOptions{
				Finder: cc.CCFinder{},
			},
			toolchains.FinderWithOptions{
				Finder: sleep.SleepToolchainFinder{},
			},
		),
		agent.WithToolchainRunners(cctoolchain.AddToStore, sleeptoolchain.AddToStore),
		agent.WithSchedulerClient(schedulerClient),
		agent.WithMonitorClient(monitorClient),
	)
	types.RegisterAgentServer(srv, a)

	mgr := servers.NewStreamManager(ctx, a)
	go mgr.Run()
	go a.StartMetricsProvider()
	err = srv.Serve(listener)
	if err != nil {
		lg.With(zap.Error(err)).Error("GRPC error")
	}
}

var Command = &cobra.Command{
	Use:   "agent",
	Short: "Run the agent service",
	Run:   run,
}
