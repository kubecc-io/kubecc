package servers

import (
	"context"
	"crypto/tls"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"

	"google.golang.org/grpc"
)

type GRPCOptions struct {
	AgentInfo   *types.AgentInfo
	tls         bool
	dialOptions []grpc.DialOption
}
type grpcOption func(*GRPCOptions)

func (o *GRPCOptions) Apply(opts ...grpcOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithAgentInfo(info *types.AgentInfo) grpcOption {
	return func(op *GRPCOptions) {
		op.AgentInfo = info
	}
}

func WithTLS(tls bool) grpcOption {
	return func(op *GRPCOptions) {
		op.tls = tls
	}
}

func With(dialOption grpc.DialOption) grpcOption {
	return func(op *GRPCOptions) {
		op.dialOptions = append(op.dialOptions, dialOption)
	}
}

func ClientAgentContextInterceptor(agentInfo *types.AgentInfo) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, resp interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = cluster.ContextWithAgentInfo(ctx, agentInfo)
		return invoker(ctx, method, req, resp, cc, opts...)
	}
}

func ServerAgentContextInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		agentInfo, err := cluster.AgentInfoFromContext(ctx)
		if err == nil {
			ctx = context.WithValue(ctx, cluster.AgentInfoKey, agentInfo)
		}
		return handler(ctx, req)
	}
}

func NewServer(ctx context.Context, opts ...grpcOption) *grpc.Server {
	options := GRPCOptions{
		AgentInfo: nil,
		tls:       false,
	}

	// Check if the context contains agent info
	if options.AgentInfo == nil {
		options.AgentInfo, _ = cluster.AgentInfoFromContext(ctx) // ignore the error
	}

	options.Apply(opts...)
	interceptors := []grpc.UnaryServerInterceptor{
		otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer()),
	}
	if options.AgentInfo != nil {
		interceptors = append(interceptors, ServerAgentContextInterceptor())
	}

	return grpc.NewServer(
		grpc.MaxRecvMsgSize(1e8), // 100MB
		grpc.ChainUnaryInterceptor(interceptors...),
	)
}

func Dial(
	ctx context.Context,
	target string,
	opts ...grpcOption,
) (*grpc.ClientConn, error) {
	options := GRPCOptions{}
	options.Apply(opts...)
	interceptors := []grpc.UnaryClientInterceptor{
		otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
	}
	dialOpts := []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(interceptors...),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1e8),
			grpc.UseCompressor(gzip.Name),
		),
	}
	if options.tls {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(
			&tls.Config{
				InsecureSkipVerify: false,
			},
		)))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	if options.AgentInfo != nil {
		interceptors = append(interceptors,
			ClientAgentContextInterceptor(options.AgentInfo))
	}
	return grpc.DialContext(ctx, target, dialOpts...)
}
