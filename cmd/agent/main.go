package main

import (
	"fmt"
	"net"

	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	log *zap.Logger
)

func init() {
	lg, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	log = lg
}

func main() {
	srv := grpc.NewServer()
	listener, err := net.Listen("tcp", fmt.Sprintf(":9090"))
	if err != nil {
		log.With(
			zap.Error(err),
		).Fatal("Error listening on socket")
	}
	agent := &agentServer{}
	types.RegisterAgentServer(srv, agent)

	cancel := connectToScheduler()
	defer cancel()

	err = srv.Serve(listener)
	if err != nil {
		log.With(zap.Error(err)).Error("GRPC error")
	}
}
