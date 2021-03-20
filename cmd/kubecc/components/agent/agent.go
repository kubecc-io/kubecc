package commands

import (
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/sleep"
	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/cc"
	ccctrl "github.com/cobalt77/kubecc/pkg/cc/controller"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
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
	monitorClient := types.NewMonitorClient(monitorCC)

	a := agent.NewAgentServer(ctx,
		agent.WithUsageLimits(&metrics.UsageLimits{
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
		agent.WithToolchainRunners(ccctrl.AddToStore, ccctrl.AddToStore),
		agent.WithSchedulerClient(schedulerClient),
		agent.WithMonitorClient(monitorClient),
	)
	go a.StartMetricsProvider()

	mgr := servers.NewStreamManager(ctx, a)
	mgr.Run()
}

var Command = &cobra.Command{
	Use:   "agent",
	Short: "Run the agent service",
	Run:   run,
}
