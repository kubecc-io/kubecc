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
	_ "google.golang.org/grpc/encoding/gzip"
)

func run(cmd *cobra.Command, args []string) {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	conf := (&config.ConfigMapProvider{}).Load(ctx, types.Consumerd).Scheduler

	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		panic(err.Error())
	}
	lg.With("addr", listener.Addr().String()).
		Info("Server listening")

	grpcServer := servers.NewServer(ctx)
	srv := scheduler.NewSchedulerServer(ctx)
	types.RegisterSchedulerServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		lg.Error(err)
	}
}

var Command = &cobra.Command{
	Use:   "scheduler",
	Short: "Run the scheduler service",
	Run:   run,
}
