package tracing

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cobalt77/kubecc/pkg/types"
	opentracing "github.com/opentracing/opentracing-go"

	jaegercfg "github.com/uber/jaeger-client-go/config"
)

func Start(component types.Component) (io.Closer, error) {
	collector, ok := os.LookupEnv("JAEGER_ENDPOINT")
	if !ok {
		return nil, errors.New("JAEGER_ENDPOINT not defined, tracing disabled")
	}
	cfg := jaegercfg.Configuration{
		ServiceName: fmt.Sprintf("kubecc-%s", component),
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
		return nil, err
	}
	opentracing.SetGlobalTracer(tracer)
	return closer, err
}
