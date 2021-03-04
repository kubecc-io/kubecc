package commands

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/cachesrv"
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
		meta.WithProvider(identity.Component, meta.WithValue(types.Cache)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)
	conf := (&config.ConfigMapProvider{}).Load(ctx).Cache

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatalw("Error listening on socket")
	}

	cacheSrv := cachesrv.NewCacheServer(ctx, conf)
	types.RegisterCacheServer(srv, cacheSrv)

	go cacheSrv.Run()
	err = srv.Serve(listener)
	if err != nil {
		lg.With(zap.Error(err)).Error("GRPC error")
	}
}

var Command = &cobra.Command{
	Use:   "cache",
	Short: "Run the cache service",
	Run:   run,
}
