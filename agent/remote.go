package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/cobalt77/kube-cc/cc"
	types "github.com/cobalt77/kube-cc/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
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

func connectToMgr() {
	cc, err := grpc.Dial(
		"kubecc-mgr.kubecc.svc.cluster.local:9090",
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: false,
		}))
	if err != nil {
		log.Fatal(err)
	}
	client := types.NewMgrClient(cc)
	ctx := context.Background()
	for {
		log.Info("Starting connection to the server")
		connect, err := client.Connect(ctx, agentInfo(), grpc.WaitForReady(true))
		if err != nil {
			log.Error(err)
		}
		log.Info("Connected to the server")
		for {
			_, err := connect.Recv()
			if err == io.EOF {
				log.Info("EOF received from the server, retrying connection")
				break
			}
		}
	}
}

func StartRemoteAgent() {
	srv := grpc.NewServer()
	port := viper.GetInt("agentPort")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	agent := &localAgentServer{}
	types.RegisterLocalAgentServer(srv, agent)

	go connectToMgr()

	err = srv.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
