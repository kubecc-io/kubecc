package meta

import (
	"context"
	"fmt"

	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
)

type MetadataKey interface {
	fmt.Stringer
}

type InitProvider interface {
	Provider
	InitialValue(Context) interface{}
}

type Provider interface {
	Key() MetadataKey
	Marshal(interface{}) string
	Unmarshal(string) (interface{}, error)
}

func Component(ctx context.Context) types.Component {
	value := ctx.Value(mdkeys.ComponentKey)
	if value == nil {
		panic("No component in context")
	}
	return value.(types.Component)
}

func UUID(ctx context.Context) string {
	value := ctx.Value(mdkeys.UUIDKey)
	if value == nil {
		panic("No uuid in context")
	}
	return value.(string)
}

func Log(ctx context.Context) *zap.SugaredLogger {
	value := ctx.Value(mdkeys.LogKey)
	if value == nil {
		panic("No logger in context")
	}
	return value.(*zap.SugaredLogger)
}

func Tracer(ctx context.Context) opentracing.Tracer {
	value := ctx.Value(mdkeys.TracingKey)
	if value == nil {
		panic("No tracer in context")
	}
	return value.(opentracing.Tracer)
}

func SystemInfo(ctx context.Context) *types.SystemInfo {
	value := ctx.Value(mdkeys.SystemInfoKey)
	if value == nil {
		panic("No system info in context")
	}
	return value.(*types.SystemInfo)
}
