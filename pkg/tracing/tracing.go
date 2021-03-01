package tracing

import (
	"context"
	"io"
	"os"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/types"
	opentracing "github.com/opentracing/opentracing-go"
	"go.uber.org/zap"

	jaegercfg "github.com/uber/jaeger-client-go/config"
)

var IsEnabled bool

func Start(ctx context.Context, component types.Component) (opentracing.Tracer, io.Closer) {
	lg := meta.Log(ctx)
	collector, ok := os.LookupEnv("JAEGER_ENDPOINT")
	if !ok {
		lg.Warn("JAEGER_ENDPOINT not defined, tracing disabled")
		return opentracing.NoopTracer{}, io.NopCloser(nil)
	}
	cfg := jaegercfg.Configuration{
		ServiceName: component.Name(),
		Disabled:    false,
		RPCMetrics:  false,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			CollectorEndpoint: collector,
		},
	}
	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		lg.With(zap.Error(err)).Error("tracing disabled")
		return opentracing.NoopTracer{}, io.NopCloser(nil)
	}
	lg.Info("Tracing enabled")
	IsEnabled = true
	return tracer, closer
}

// type tracerKeyType struct{}

// var tracerKey tracerKeyType

// func ContextWithTracer(ctx context.Context, tracer opentracing.Tracer) context.Context {
// 	return context.WithValue(ctx, tracerKey, tracer)
// }

// func TracerFromContext(ctx context.Context) opentracing.Tracer {
// 	tracer := ctx.Value(tracerKey)
// 	if tracer == nil {
// 		panic("No tracer associated with context")
// 	}
// 	return tracer.(opentracing.Tracer)
// }

type tracingProvider struct{}

var Tracer tracingProvider

func (tracingProvider) Key() meta.MetadataKey {
	return mdkeys.TracingKey
}

func (tracingProvider) InitialValue(ctx meta.Context) interface{} {
	tracer, closer := Start(ctx, meta.Component(ctx))
	go func() {
		<-ctx.Done()
		closer.Close()
	}()
	return tracer
}

func (tracingProvider) Marshal(i interface{}) string {
	return ""
}

func (tracingProvider) Unmarshal(s string) interface{} {
	return nil
}
