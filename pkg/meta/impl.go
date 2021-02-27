package meta

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type contextImpl struct {
	context.Context
	providers map[interface{}]Provider
}

func (ci *contextImpl) Component() types.Component {
	if p, ok := ci.providers[mdkeys.ComponentKey]; ok {
		return ci.Value(p.Key().String()).(types.Component)
	}
	panic("No component in context")
}

func (ci *contextImpl) UUID() string {
	if p, ok := ci.providers[mdkeys.UuidKey]; ok {
		return ci.Value(p.Key().String()).(string)
	}
	panic("No uuid in context")
}

func (ci *contextImpl) Log() *zap.SugaredLogger {
	if p, ok := ci.providers[mdkeys.LogKey.String()]; ok {
		return ci.Value(p.Key().String()).(*zap.SugaredLogger)
	}
	panic("No logger in context")
}

func (ci *contextImpl) Tracer() opentracing.Tracer {
	if p, ok := ci.providers[mdkeys.TracingKey.String()]; ok {
		return ci.Value(p.Key().String()).(opentracing.Tracer)
	}
	panic("No logger in context")
}

func (c *contextImpl) ImportFromIncoming(ctx context.Context, md metadata.MD) {
	for _, mp := range c.providers {
		mp.ImportFromIncoming(c, md)
	}
}

func (c *contextImpl) ExportToOutgoing(ctx context.Context) {
	for _, mp := range c.providers {
		mp.ExportToOutgoing(c)
	}
}

func (c *contextImpl) MetadataProviders() []Provider {
	providers := []Provider{}
	for _, p := range c.providers {
		providers = append(providers, p)
	}
	return providers
}
