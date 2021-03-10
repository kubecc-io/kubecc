package main

import (
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/apps/consumer"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
)

func main() {
	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumer)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(types.Consumer,
				logkc.WithOutputPaths([]string{"/tmp/consumer.log"}),
				logkc.WithErrorOutputPaths([]string{"/tmp/consumer.log"}),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	conf := (&config.ConfigMapProvider{}).Load(ctx).Consumer

	cc, err := servers.Dial(ctx, conf.ConsumerdAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error connecting to leader")
	}
	consumer.DispatchAndWait(ctx, cc)
}
