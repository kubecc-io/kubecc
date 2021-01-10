package main

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type remoteAgentServer struct {
	types.RemoteAgentServer
}

func (s *remoteAgentServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	info := cc.NewArgsInfo(req.Command, req.Args, log)
	out, err := cc.Run(info, cc.WithCompressOutput())
	if err != nil {
		return nil, err
	}
	return &types.CompileResponse{
		CompiledSource: out,
	}, nil
}

func connectToScheduler() context.CancelFunc {
	ctx, cancel := cluster.NewAgentContext()
	go func() {
		cc, err := grpc.Dial(
			fmt.Sprintf("kubecc-scheduler.%s.svc.cluster.local:9090",
				cluster.GetNamespace()),
			grpc.WithInsecure())
		if err != nil {
			log.With(zap.Error(err)).Fatal("Error dialing scheduler")
		}
		client := types.NewSchedulerClient(cc)
		for {
			log.Info("Starting connection to the scheduler")
			stream, err := client.Connect(ctx, grpc.WaitForReady(true))
			if err != nil {
				log.With(zap.Error(err)).Error("Error connecting to scheduler")
			}
			log.Info("Connected to the scheduler")
			for {
				_, err := stream.Recv()
				if err == io.EOF {
					log.Info("EOF received from the scheduler, retrying connection")
					break
				}
			}
		}
	}()
	return cancel
}

func startRemoteAgent() {
	srv := grpc.NewServer()
	port := viper.GetInt("agentPort")
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.With(
			zap.Error(err),
			zap.Int("port", port),
		).Fatal("Error listening on socket")
	}
	agent := &localAgentServer{}
	types.RegisterLocalAgentServer(srv, agent)

	cancel := connectToScheduler()
	defer cancel()

	err = srv.Serve(listener)
	if err != nil {
		log.With(zap.Error(err)).Error("GRPC error")
	}
}
