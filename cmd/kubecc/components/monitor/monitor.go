package commands

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

func run(cmd *cobra.Command, args []string) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	conf := (&config.ConfigMapProvider{}).Load(ctx).Monitor

	extListener, err := net.Listen("tcp", conf.ListenAddress.External)
	if err != nil {
		panic(err.Error())
	}
	lg.With("addr", extListener.Addr().String()).Info("External API listening")

	intListener, err := net.Listen("tcp", conf.ListenAddress.Internal)
	if err != nil {
		panic(err.Error())
	}
	lg.With("addr", intListener.Addr().String()).Info("Internal API listening")

	internal := servers.NewServer(ctx)
	external := servers.NewServer(ctx)
	srv := monitor.NewMonitorServer(ctx, monitor.InMemoryStoreCreator)
	types.RegisterInternalMonitorServer(internal, srv)
	types.RegisterExternalMonitorServer(external, srv)

	go func() {
		err = external.Serve(extListener)
		if err != nil {
			lg.Error(err)
		}
	}()
	err = internal.Serve(intListener)
	if err != nil {
		lg.Error(err)
	}
}

var Command = &cobra.Command{
	Use:   "monitor",
	Short: "Run the monitor service",
	Run:   run,
}
