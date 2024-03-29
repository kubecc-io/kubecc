/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// Package servers contains helper functions and types for dealing with
// gRPC servers and streams.
package servers

import (
	"context"
	"crypto/x509"
	"math"

	"github.com/kralicky/grpc-opentracing/go/otgrpc"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"

	"google.golang.org/grpc"
)

type GRPCOptions struct {
	tls           bool
	dialOptions   []grpc.DialOption
	serverOptions []grpc.ServerOption
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

func WithDialOpts(dialOptions ...grpc.DialOption) grpcOption {
	return func(op *GRPCOptions) {
		op.dialOptions = append(op.dialOptions, dialOptions...)
	}
}

func WithServerOpts(serverOptions ...grpc.ServerOption) grpcOption {
	return func(op *GRPCOptions) {
		op.serverOptions = append(op.serverOptions, serverOptions...)
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
		append(options.serverOptions,
			grpc.MaxRecvMsgSize(1e8), // 100MB
			grpc.ChainUnaryInterceptor(
				otgrpc.OpenTracingServerInterceptor(meta.Tracer(ctx)),
				meta.ServerContextInterceptor(importOptions),
			),
			grpc.ChainStreamInterceptor(
				meta.StreamServerContextInterceptor(importOptions),
			),
		)...,
	)
}

func Dial(
	ctx context.Context,
	target string,
	opts ...grpcOption,
) (*grpc.ClientConn, error) {
	options := GRPCOptions{}
	options.Apply(opts...)
	dialOpts := append(options.dialOptions,
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
			grpc.MaxCallRecvMsgSize(math.MaxInt32),
			// note: this maybe causes massive slowdowns when used with anypb
			// grpc.UseCompressor(gzip.Name),
		),
	)
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
