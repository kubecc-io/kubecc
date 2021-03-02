package meta

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
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
		if c, ok := ctx.(Context); ok {
			ctx = c.ExportToOutgoing()
		} else if mctx, ok := FromContext(ctx); ok {
			ctx = mctx.ExportToOutgoing()
		}
		return invoker(ctx, method, req, resp, cc, opts...)
	}
}

func ServerContextInterceptor(
	srvCtx Context,
	options ImportOptions,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		c := &contextImpl{
			providers: make(map[interface{}]Provider),
			values:    make(map[interface{}]interface{}),
		}
		// Import client providers
		err = c.ImportFromIncoming(ctx, options)
		if err != nil {
			return nil, err
		}
		// Any remaining providers are taken from the server context
		for _, p := range srvCtx.MetadataProviders() {
			if _, ok := c.providers[p.Key()]; !ok {
				c.providers[p.Key()] = p
				c.values[p.Key()] = srvCtx.Value(p.Key())
			}
		}
		// Add back the grpc peer
		srvContext := context.Context(c)
		if p, ok := peer.FromContext(ctx); ok {
			srvContext = peer.NewContext(srvContext, p)
		}
		return handler(srvContext, req)
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
		if c, ok := ctx.(Context); ok {
			ctx = c.ExportToOutgoing()
		} else if mctx, ok := FromContext(ctx); ok {
			ctx = mctx.ExportToOutgoing()
		}
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
	ctx Context,
	options ImportOptions,
) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		c := &contextImpl{
			providers: make(map[interface{}]Provider),
			values:    make(map[interface{}]interface{}),
		}
		// Import client providers
		err := c.ImportFromIncoming(ss.Context(), options)
		if err != nil {
			return err
		}
		// Any remaining providers are taken from the server context
		for _, p := range ctx.MetadataProviders() {
			if _, ok := c.providers[p.Key()]; !ok {
				c.providers[p.Key()] = p
				c.values[p.Key()] = ctx.Value(p.Key())
			}
		}
		// Add back the grpc peer
		srvContext := context.Context(c)
		if p, ok := peer.FromContext(ss.Context()); ok {
			srvContext = peer.NewContext(srvContext, p)
		}
		return handler(srv, &serverStream{
			ServerStream: ss,
			ctx:          srvContext,
		})
	}
}
