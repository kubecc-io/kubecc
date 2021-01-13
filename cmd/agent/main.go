package main

import (
	"fmt"
	"net"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	lll.Setup("A")
	lll.PrintHeader()

	srv := grpc.NewServer(
		// grpc.MaxConcurrentStreams(uint32(runtime.NumCPU()*3)/2),
		grpc.MaxRecvMsgSize(5e7), // 50MB
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
