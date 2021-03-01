package meta

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type metaContextKeyType struct{}

var metaContextKey metaContextKeyType

type contextImpl struct {
	providers map[interface{}]Provider
	values    map[interface{}]interface{}
}

func (*contextImpl) Deadline() (deadline time.Time, ok bool) {
	return
}

func (*contextImpl) Done() <-chan struct{} {
	return nil
}

func (*contextImpl) Err() error {
	return nil
}

func (ci *contextImpl) Value(key interface{}) interface{} {
	if key == metaContextKey {
		return ci
	}
	if value, ok := ci.values[key]; ok {
		return value
	}
	return nil
}

func (*contextImpl) String() string {
	return "empty meta.Context"
}

type ImportOptions struct {
	Required []Provider
	Optional []Provider
}

func (c *contextImpl) ImportFromIncoming(ctx context.Context, opts ImportOptions) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic("Not an incoming context")
	}
	for _, mp := range opts.Required {
		if values := md.Get(mp.Key().String()); len(values) > 0 {
			c.providers[mp.Key()] = mp
			var err error
			c.values[mp.Key()], err = mp.Unmarshal(values[0])
			if err != nil {
				return status.Error(codes.InvalidArgument,
					fmt.Sprintf("Failed to unmarshal required metadata for provider %s: %s",
						reflect.TypeOf(mp), err.Error()))
			}
		} else {
			return status.Error(codes.InvalidArgument,
				fmt.Sprintf("Expected metadata provider %s was not found",
					reflect.TypeOf(mp)))
		}
	}
	for _, mp := range opts.Optional {
		if values := md.Get(mp.Key().String()); len(values) > 0 {
			c.providers[mp.Key()] = mp
			var err error
			c.values[mp.Key()], err = mp.Unmarshal(values[0])
			if err != nil {
				return status.Error(codes.InvalidArgument,
					fmt.Sprintf("Failed to unmarshal optional metadata for provider %s: %s",
						reflect.TypeOf(mp), err.Error()))
			}
		}
	}
	return nil
}

func (c *contextImpl) ExportToOutgoing() context.Context {
	kv := []string{}
	for _, mp := range c.providers {
		kv = append(kv,
			mp.Key().String(),
			mp.Marshal(c.Value(mp.Key())),
		)
	}
	return metadata.AppendToOutgoingContext(c, kv...)
}

func (c *contextImpl) MetadataProviders() []Provider {
	providers := []Provider{}
	for _, v := range c.providers {
		providers = append(providers, v)
	}
	return providers
}

func FromContext(ctx context.Context) (Context, bool) {
	if mctx := ctx.Value(metaContextKey); mctx != nil {
		return mctx.(Context), true
	}
	return nil, false
}
