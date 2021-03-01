package main

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/agent"
	"github.com/cobalt77/kubecc/pkg/host"
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
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		lg.With(zap.Error(err)).Fatalw("Error listening on socket")
	}
	a := agent.NewAgentServer(ctx)
	types.RegisterAgentServer(srv, a)
	go a.RunSchedulerClient()
	err = srv.Serve(listener)
	if err != nil {
		lg.With(zap.Error(err)).Error("GRPC error")
	}
}
