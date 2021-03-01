package main

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/monitor"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
)

var lg *zap.SugaredLogger

func main() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Monitor)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	logkc.PrintHeader()

	extListener, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err.Error())
	}
	lg.With("addr", extListener.Addr().String()).Info("External API listening")

	intListener, err := net.Listen("tcp", ":9091")
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
