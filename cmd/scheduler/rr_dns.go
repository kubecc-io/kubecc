package main

import (
	"fmt"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
)

type dnsResolver struct {
	AgentResolver

	balancer string
}

func NewDnsResolver(balancer string) AgentResolver {
	return &dnsResolver{
		balancer: balancer,
	}
}

func (r *dnsResolver) Dial() (*grpc.ClientConn, error) {
	ns := cluster.GetNamespace()
	serviceAddr := fmt.Sprintf("dns:///kubecc-agent.%s.svc.cluster.local:9090", ns)
	return grpc.Dial(serviceAddr, grpc.WithInsecure(),
		grpc.WithBalancerName(r.balancer))
}

func init() {
	roundRobinDns := &DefaultScheduler{
		resolver: NewDnsResolver(roundrobin.Name),
	}
	AddScheduler("roundRobinDns", roundRobinDns)
}
