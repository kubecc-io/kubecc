package commands

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	cctoolchain "github.com/cobalt77/kubecc/pkg/cc/toolchain"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func run(cmd *cobra.Command, args []string) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)

	conf := (&config.ConfigMapProvider{}).Load(ctx, types.Consumerd).Consumerd

	schedulerCC, err := servers.Dial(ctx, conf.SchedulerAddress,
		servers.WithTLS(!conf.disableTLS),
	)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing scheduler")
	}

	lg.With("address", schedulerCC.Target()).Info("Dialing scheduler")
	schedulerClient := types.NewSchedulerClient(schedulerCC)

	d := consumerd.NewConsumerdServer(ctx,
		consumerd.WithUsageLimits(&types.UsageLimits{
			ConcurrentProcessLimit:  int32(conf.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: conf.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   conf.UsageLimits.QueueRejectMultiplier,
		}),
		consumerd.WithToolchainRunners(cctoolchain.AddToStore),
		consumerd.WithSchedulerClient(schedulerClient, schedulerCC),
	)

	mgr := servers.NewStreamManager(ctx, d)
	go mgr.Run()

	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		lg.With(
			zap.Error(err),
			zap.String("address", conf.ListenAddress),
		).Fatal("Error listening on socket")
	}
	lg.With("addr", listener.Addr()).Info("Listening")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Could not start consumerd")
	}

	srv := servers.NewServer(ctx)
	types.RegisterConsumerdServer(srv, d)
	err = srv.Serve(listener)
	if err != nil {
		lg.Error(err.Error())
	}
}

var Command = &cobra.Command{
	Use:   "consumerd",
	Short: "Run the consumerd service",
	Run:   run,
}
