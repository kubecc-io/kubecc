package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cobalt77/kubecc/pkg/types"
)

type RoundRobinScheduler struct {
	AgentScheduler

	agentClient types.RemoteAgentClient
}

func NewRoundRobinScheduler() (AgentScheduler, error) {
	ns, ok := os.LookupEnv("KUBECC_NAMESPACE")
	if !ok {
		log.Fatal("KUBECC_NAMESPACE not defined")
	}
	serviceAddr := fmt.Sprintf("dns://kubecc-agent.%s.svc.cluster.local", ns)
	cc, err := grpc.Dial(serviceAddr, grpc.WithInsecure(),
		grpc.WithBalancerName(roundrobin.Name))
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &RoundRobinScheduler{
		agentClient: types.NewRemoteAgentClient(cc),
	}, nil
}

func (s *RoundRobinScheduler) Schedule(
	req *types.CompileRequest,
) (*CompileTask, error) {
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := s.agentClient.Compile(ctx, req)
	if err != nil {
		return nil, err
	}
	statusCh := make(chan *types.CompileStatus)
	errorCh := make(chan error)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				errorCh <- err
				break
			}
			statusCh <- msg
		}
	}()
	return &CompileTask{
		Status: statusCh,
		Error:  errorCh,
		Cancel: cancel,
	}, nil
}
