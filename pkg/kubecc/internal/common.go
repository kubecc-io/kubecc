package internal

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
)

var (
	CLIContext context.Context
	CLILog     *zap.SugaredLogger
	CLIConfig  config.KcctlSpec
)

func init() {
	CLIConfig = (&config.ConfigMapProvider{}).Load().Kcctl

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.CLI,
				logkc.WithLogLevel(CLIConfig.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	CLIContext = ctx
	CLILog = lg
}
