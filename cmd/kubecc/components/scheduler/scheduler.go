package commands

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	_ "google.golang.org/grpc/encoding/gzip"
)

func run(cmd *cobra.Command, args []string) {
	conf := (&config.ConfigMapProvider{}).Load().Scheduler

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.Scheduler,
				logkc.WithLogLevel(conf.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		panic(err.Error())
	}
	lg.With("addr", listener.Addr().String()).
		Info("Server listening")

	monitorCC, err := servers.Dial(ctx, conf.MonitorAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing monitor")
	}
	lg.With("address", monitorCC.Target()).Info("Dialing monitor")

	monitorClient := types.NewInternalMonitorClient(monitorCC)

	sc := scheduler.NewSchedulerServer(ctx,
		scheduler.WithMonitorClient(monitorClient),
	)
	types.RegisterSchedulerServer(srv, sc)
	go sc.StartMetricsProvider()

	err = srv.Serve(listener)
	if err != nil {
		lg.With(zap.Error(err)).Error("GRPC error")
	}
}

var Command = &cobra.Command{
	Use:   "scheduler",
	Short: "Run the scheduler service",
	Run:   run,
}
