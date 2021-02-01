package main

import (
	"net"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	lll.Setup("S")
	lll.PrintHeader()

	lll.Info("Server starting")
	initConfig()

	closer, err := tracing.Start("scheduler")
	if err != nil {
		lll.With(zap.Error(err)).Warn("Could not start tracing")
	} else {
		defer closer.Close()
	}

	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err.Error())
	}
	lll.With("addr", listener.Addr().String()).
		Info("Server listening")

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(1e8), // 100MB
		grpc.UnaryInterceptor(
			otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
	)

	srv := NewSchedulerServer()
	types.RegisterSchedulerServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		lll.Error(err)
	}
}
