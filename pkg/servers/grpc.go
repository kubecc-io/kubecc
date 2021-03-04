package servers

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/cobalt77/grpc-opentracing/go/otgrpc"
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding/gzip"

	"google.golang.org/grpc"
)

type GRPCOptions struct {
	tls         bool
	dialOptions []grpc.DialOption
}
type grpcOption func(*GRPCOptions)

func (o *GRPCOptions) Apply(opts ...grpcOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithTLS(tls bool) grpcOption {
	return func(op *GRPCOptions) {
		op.tls = tls
	}
}

func With(dialOption ...grpc.DialOption) grpcOption {
	return func(op *GRPCOptions) {
		op.dialOptions = append(op.dialOptions, dialOption...)
	}
}

func NewServer(ctx context.Context, opts ...grpcOption) *grpc.Server {
	options := GRPCOptions{
		tls: false,
	}
	options.Apply(opts...)

	importOptions := meta.ImportOptions{
		Required: []meta.Provider{
			identity.Component,
			identity.UUID,
		},
		Optional: []meta.Provider{
			host.SystemInfo,
		},
		Inherit: &meta.InheritOptions{
			InheritFrom: ctx,
			Providers: []meta.Provider{
				logkc.Logger,
				tracing.Tracer,
			},
		},
	}

	return grpc.NewServer(
		grpc.MaxRecvMsgSize(1e8), // 100MB
		grpc.ChainUnaryInterceptor(
			otgrpc.OpenTracingServerInterceptor(meta.Tracer(ctx)),
			meta.ServerContextInterceptor(importOptions),
		),
		grpc.ChainStreamInterceptor(
			meta.StreamServerContextInterceptor(importOptions),
		),
	)
}

func DialService(
	ctx context.Context,
	component types.Component,
	port int,
	opts ...grpcOption,
) (*grpc.ClientConn, error) {
	addr := fmt.Sprintf("kubecc-%s.%s.svc.cluster.local:%d",
		strings.ToLower(component.Name()),
		cluster.GetNamespace(),
		port,
	)
	return Dial(ctx, addr, opts...)
}

func Dial(
	ctx context.Context,
	target string,
	opts ...grpcOption,
) (*grpc.ClientConn, error) {
	options := GRPCOptions{}
	options.Apply(opts...)
	dialOpts := append([]grpc.DialOption{
		grpc.WithChainUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(
				meta.Tracer(ctx),
				otgrpc.CreateSpan(!tracing.IsEnabled),
			),
			meta.ClientContextInterceptor(),
		),
		grpc.WithChainStreamInterceptor(
			meta.StreamClientContextInterceptor(),
		),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1e8),
			grpc.UseCompressor(gzip.Name),
		),
	}, options.dialOptions...)
	if options.tls {
		pool, err := x509.SystemCertPool()
		if err != nil {
			meta.Log(ctx).With(zap.Error(err)).Fatal("Error reading system cert pool")
		}
		dialOpts = append(dialOpts,
			grpc.WithTransportCredentials(
				credentials.NewClientTLSFromCert(pool, "")))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}

	return grpc.DialContext(ctx, target, dialOpts...)
}

func StartSpanFromServer(
	clientCtx context.Context,
	operationName string,
) (opentracing.Span, context.Context, error) {
	tracer := meta.Tracer(clientCtx)
	if tracer == nil {
		panic("No tracer in server context")
	}

	if !tracing.IsEnabled {
		span := tracer.StartSpan(operationName)
		ctx := opentracing.ContextWithSpan(clientCtx, span)
		return span, ctx, nil
	}

	spanContext, err := otgrpc.ExtractSpanContext(clientCtx, tracer)
	if err != nil {
		return nil, nil, err
	}
	span := tracer.StartSpan(operationName, ext.RPCServerOption(spanContext))
	ctx := opentracing.ContextWithSpan(clientCtx, span)
	return span, ctx, nil
}
