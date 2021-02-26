package main

import (
	"context"
	"fmt"
	"net"

	"github.com/cobalt77/kubecc/internal/consumer"
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	cctoolchain "github.com/cobalt77/kubecc/pkg/cc/toolchain"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var lg *zap.SugaredLogger

func main() {
	ctx := logkc.NewWithContext(context.Background(), types.Consumerd,
		logkc.WithLogLevel(zapcore.DebugLevel),
	)
	logkc.PrintHeader()

	consumer.InitConfig()
	tracer, closer := tracing.Start(ctx, types.Consumerd)
	defer closer.Close()
	ctx = tracing.ContextWithTracer(ctx, tracer)

	d := consumerd.NewConsumerdServer(ctx,
		consumerd.WithToolchainRunners(cctoolchain.AddToStore))

	d.ConnectToRemote()
	go d.RunSchedulerClient()

	port := viper.GetInt("port")
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		lg.With(zap.Error(err), zap.Int("port", port)).
			Fatal("Error listening on socket")
	}
	lg.With("addr", listener.Addr()).Info("Listening")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Could not start consumerd")
	}

	srv := servers.NewServer(ctx)
	types.RegisterConsumerdServer(srv, d)
	err = srv.Serve(listener)
	if err != nil {
		lg.Error(err.Error())
	}
}
