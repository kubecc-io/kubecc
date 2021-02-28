package main

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	_ "google.golang.org/grpc/encoding/gzip"
)

var lg *zap.SugaredLogger

func main() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.MetadataProvider),
		meta.WithProvider(tracing.MetadataProvider),
	)
	lg := ctx.Log()

	logkc.PrintHeader()
	listener, err := net.Listen("tcp", ":9090")
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
