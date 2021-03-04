package meta

import (
	"context"

	"google.golang.org/grpc"
)

func ClientContextInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, resp interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = ExportToOutgoing(ctx)
		return invoker(ctx, method, req, resp, cc, opts...)
	}
}

func ServerContextInterceptor(
	options ImportOptions,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		ctx, err = ImportFromIncoming(ctx, options)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func StreamClientContextInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx = ExportToOutgoing(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (ss *serverStream) Context() context.Context {
	return ss.ctx
}

func StreamServerContextInterceptor(
	options ImportOptions,
) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, err := ImportFromIncoming(ss.Context(), options)
		if err != nil {
			return err
		}
		return handler(srv, &serverStream{
			ServerStream: ss,
			ctx:          ctx,
		})
	}
}
