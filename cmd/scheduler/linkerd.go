package main

import (
	"fmt"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"google.golang.org/grpc"
)

type linkerdResolver struct {
	AgentResolver
}

func NewLinkerdResolver() AgentResolver {
	return &linkerdResolver{}
}

func (r *linkerdResolver) Dial() (*grpc.ClientConn, error) {
	ns := cluster.GetNamespace()
	serviceAddr := fmt.Sprintf("kubecc-agent.%s.svc.cluster.local:9090", ns)
	return grpc.Dial(serviceAddr, grpc.WithInsecure())
}

func init() {
	linkerd := &DefaultScheduler{
		resolver: NewLinkerdResolver(),
	}
	AddScheduler("linkerd", linkerd)
}
