package main

import (
	"fmt"
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
	lll.Setup("A")
	lll.PrintHeader()
	closer, err := tracing.Start("agent")
	if err != nil {
		defer closer.Close()
	}
	srv := grpc.NewServer(
		// grpc.MaxConcurrentStreams(uint32(runtime.NumCPU()*3)/2),
		grpc.MaxRecvMsgSize(1e8), // 100MB
		grpc.UnaryInterceptor(
			otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
	)
	listener, err := net.Listen("tcp", fmt.Sprintf(":9090"))
	if err != nil {
		lll.With(
			zap.Error(err),
		).Fatal("Error listening on socket")
	}
	agent := NewAgentServer()
	types.RegisterAgentServer(srv, agent)
	// connectToScheduler()
	err = srv.Serve(listener)
	if err != nil {
		lll.With(zap.Error(err)).Error("GRPC error")
	}
}
