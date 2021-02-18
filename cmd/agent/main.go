package main

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	_ "google.golang.org/grpc/encoding/gzip"
)

var lg *zap.SugaredLogger

func main() {
	ctx := logkc.NewWithContext(cluster.NewAgentContext(), types.Agent)
	lg = logkc.LogFromContext(ctx)

	logkc.PrintHeader()
	tracer, closer := tracing.Start(ctx, types.Agent)
	defer closer.Close()
	ctx = tracing.ContextWithTracer(ctx, tracer)

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		lg.With(zap.Error(err)).Fatalw("Error listening on socket")
	}
	a := agent.NewAgentServer(ctx)
	types.RegisterAgentServer(srv, a)
	go a.RunSchedulerClient(ctx)
	err = srv.Serve(listener)
	if err != nil {
		lg.With(zap.Error(err)).Error("GRPC error")
	}
}
