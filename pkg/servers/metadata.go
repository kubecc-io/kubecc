package servers

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc"
)

func ClientContextInterceptor(agentInfo *types.AgentInfo) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, resp interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx.
		return invoker(ctx, method, req, resp, cc, opts...)
	}
}

func ServerContextInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		agentInfo, err := cluster.AgentInfoFromContext(ctx)
		if err == nil {
			ctx = context.WithValue(ctx, cluster.AgentInfoKey, agentInfo)
		}
		return handler(ctx, req)
	}
}
