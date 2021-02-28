package meta

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
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

func (ci *contextImpl) Component() types.Component {
	if p, ok := ci.providers[mdkeys.ComponentKey]; ok {
		return ci.Value(p.Key()).(types.Component)
	}
	panic("No component in context")
}

func (ci *contextImpl) UUID() string {
	if p, ok := ci.providers[mdkeys.UUIDKey]; ok {
		return ci.Value(p.Key()).(string)
	}
	panic("No uuid in context")
}

func (ci *contextImpl) Log() *zap.SugaredLogger {
	if p, ok := ci.providers[mdkeys.LogKey]; ok {
		return ci.Value(p.Key()).(*zap.SugaredLogger)
	}
	panic("No logger in context")
}

func (ci *contextImpl) Tracer() opentracing.Tracer {
	if p, ok := ci.providers[mdkeys.TracingKey]; ok {
		return ci.Value(p.Key()).(opentracing.Tracer)
	}
	panic("No logger in context")
}

func (c *contextImpl) ImportFromIncoming(ctx context.Context, expected ...Provider) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic("Not an incoming context")
	}
	for _, mp := range expected {
		if values := md.Get(mp.Key().String()); len(values) > 0 {
			c.providers[mp.Key()] = mp
			c.values[mp.Key()] = mp.Unmarshal(values[0])
		} else {
			panic(fmt.Sprintf("Expected metadata provider %s was not found",
				reflect.TypeOf(mp)))
		}
	}
	return
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
