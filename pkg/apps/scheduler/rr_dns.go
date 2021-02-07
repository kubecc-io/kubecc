package scheduler

import (
	"fmt"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
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
	return grpc.Dial(
		serviceAddr,
		grpc.WithInsecure(),
		grpc.WithBalancerName(r.balancer),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1e8)), // 100 MB
	)
}
