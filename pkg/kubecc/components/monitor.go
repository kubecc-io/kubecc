package components

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
)

func runMonitor(cmd *cobra.Command, args []string) {
	conf := (&config.ConfigMapProvider{}).Load().Monitor

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.Monitor,
				logkc.WithLogLevel(conf.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		panic(err.Error())
	}
	lg.With("addr", listener.Addr().String()).Info("Metrics API listening")

	srv := servers.NewServer(ctx)
	monitorServer := monitor.NewMonitorServer(ctx, conf, monitor.InMemoryStoreCreator)
	types.RegisterMonitorServer(srv, monitorServer)

	err = srv.Serve(listener)
	if err != nil {
		lg.Error(err)
	}
}

var MonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Run the monitor server",
	Run:   runMonitor,
}
