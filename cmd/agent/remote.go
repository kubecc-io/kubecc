package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

type remoteAgentServer struct {
	types.RemoteAgentServer
}

func (s *remoteAgentServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	info := cc.NewArgsInfo(req.Command, req.Args)
	out, err := cc.Run(info, cc.WithCompressOutput())
	if err != nil {
		return nil, err
	}
	return &types.CompileResponse{
		CompiledSource: out,
	}, nil
}

func agentInfo() *types.AgentInfo {
	host, err := os.Hostname()
	if err != nil {
		host = "<unknown>"
		log.Warning("Could not determine hostname")
	}
	return &types.AgentInfo{
		Arch:     runtime.GOARCH,
		Hostname: host,
		NumCpus:  int32(runtime.NumCPU()),
	}
}

func connectToScheduler() {
	cc, err := grpc.Dial(
		"kubecc-scheduler.kubecc-system.svc.cluster.local:9090",
		grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	client := types.NewSchedulerClient(cc)
	ctx := context.Background()
	for {
		log.Info("Starting connection to the server")
		stream, err := client.Connect(ctx, grpc.WaitForReady(true))
		if err != nil {
			log.Error(err)
		}
		log.Info("Connected to the server")
		stream.Send(agentInfo())
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				log.Info("EOF received from the server, retrying connection")
				break
			}
		}
	}
}

func startRemoteAgent() {
	srv := grpc.NewServer()
	port := viper.GetInt("agentPort")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	agent := &localAgentServer{}
	types.RegisterLocalAgentServer(srv, agent)

	go connectToScheduler()

	err = srv.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
