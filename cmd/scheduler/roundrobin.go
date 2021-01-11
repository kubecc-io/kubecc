package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
)

type RoundRobinScheduler struct {
	AgentScheduler

	agentClient types.AgentClient
}

func NewRoundRobinScheduler() (AgentScheduler, error) {
	log.Info("Starting round-robin scheduler")
	ns := cluster.GetNamespace()
	serviceAddr := fmt.Sprintf("dns:///kubecc-agent.%s.svc.cluster.local:9090", ns)
	log.Infof("Load balancing at %s", serviceAddr)
	cc, err := grpc.Dial(serviceAddr, grpc.WithInsecure(),
		grpc.WithBalancerName(roundrobin.Name))
	if err != nil {
		log.With(zap.Error(err)).Error("Error dialing service")
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &RoundRobinScheduler{
		agentClient: types.NewAgentClient(cc),
	}, nil
}

func (s *RoundRobinScheduler) Schedule(
	req *types.CompileRequest,
) (*CompileTask, error) {
	log.Info("Scheduling using round-robin")
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := s.agentClient.Compile(ctx, req, grpc.UseCompressor(gzip.Name))
	if err != nil {
		log.With(zap.Error(err)).Error("Error from agent")
		cancel()
		return nil, err
	}
	log.Info("Agent started task")
	return NewCompileTask(stream, cancel), nil
}
