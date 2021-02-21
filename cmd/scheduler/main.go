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
	ctx := logkc.NewWithContext(context.Background(), types.Scheduler)
	lg = logkc.LogFromContext(ctx)
	logkc.PrintHeader()

	tracer, closer := tracing.Start(ctx, types.Scheduler)
	defer closer.Close()
	ctx = tracing.ContextWithTracer(ctx, tracer)

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
