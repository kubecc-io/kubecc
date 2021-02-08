package main

import (
	"context"
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/scheduler"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	_ "google.golang.org/grpc/encoding/gzip"
)

var lg *zap.SugaredLogger

func main() {
	ctx := logkc.NewFromContext(context.Background(), types.Scheduler)
	lg = logkc.LogFromContext(ctx)
	logkc.PrintHeader()

	closer, err := tracing.Start(types.Scheduler)
	if err != nil {
		lg.With(zap.Error(err)).Warn("Could not start tracing")
	} else {
		lg.Info("Tracing started successfully")
		defer closer.Close()
	}

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
