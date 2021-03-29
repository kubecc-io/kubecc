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

package tracing

import (
	"context"
	"io"
	"os"

	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/meta/mdkeys"
	"github.com/kubecc-io/kubecc/pkg/types"
	opentracing "github.com/opentracing/opentracing-go"
	"go.uber.org/zap"

	jaegercfg "github.com/uber/jaeger-client-go/config"
)

var IsEnabled bool

func Start(ctx context.Context, component types.Component) (opentracing.Tracer, io.Closer) {
	lg := meta.Log(ctx)
	collector, ok := os.LookupEnv("JAEGER_ENDPOINT")
	if !ok {
		lg.Info("JAEGER_ENDPOINT not defined, tracing disabled")
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

type tracingProvider struct{}

var Tracer tracingProvider

func (tracingProvider) Key() meta.MetadataKey {
	return mdkeys.TracingKey
}

func (tracingProvider) InitialValue(ctx context.Context) interface{} {
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

func (tracingProvider) Unmarshal(s string) (interface{}, error) {
	return nil, nil
}
