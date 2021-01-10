package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
)

type RoundRobinScheduler struct {
	AgentScheduler

	agentClient types.AgentClient
}

func NewRoundRobinScheduler() (AgentScheduler, error) {
	ns := cluster.GetNamespace()
	serviceAddr := fmt.Sprintf("dns://kubecc-agent.%s.svc.cluster.local", ns)
	cc, err := grpc.Dial(serviceAddr, grpc.WithInsecure(),
		grpc.WithBalancerName(roundrobin.Name))
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &RoundRobinScheduler{
		agentClient: types.NewAgentClient(cc),
	}, nil
}

func (s *RoundRobinScheduler) Schedule(
	req *types.CompileRequest,
) (*CompileTask, error) {
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := s.agentClient.Compile(ctx, req)
	if err != nil {
		cancel()
		return nil, err
	}
	return NewCompileTask(stream, cancel), nil
}
