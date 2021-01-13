package main

import (
	"net"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

func main() {
	lll.Setup("S")
	lll.PrintHeader()

	lll.Info("Server starting")
	initConfig()

	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err.Error())
	}
	lll.With("addr", listener.Addr().String()).
		Info("Server listening")

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(5e7), // 50MB
	)

	srv := NewSchedulerServer()
	types.RegisterSchedulerServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		lll.Error(err)
	}
}
