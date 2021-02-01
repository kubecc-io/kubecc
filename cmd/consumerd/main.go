package main

import (
	"github.com/cobalt77/kubecc/internal/consumer"
	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	lll.Setup("D", lll.WithLogLevel(zapcore.DebugLevel))
	lll.PrintHeader()

	consumer.InitConfig()
	closer, err := tracing.Start("consumerd")
	if err != nil {
		lll.With(zap.Error(err)).Warn("Could not start tracing")
	} else {
		defer closer.Close()
	}
	startConsumerd()
}
