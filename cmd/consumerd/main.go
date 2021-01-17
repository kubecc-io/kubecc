package main

import (
	"github.com/cobalt77/kubecc/internal/consumer"
	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/tracing"
)

func main() {
	lll.Setup("D")
	lll.PrintHeader()

	consumer.InitConfig()
	closer, err := tracing.Start("consumerd")
	if err != nil {
		defer closer.Close()
	}
	startConsumerd()
}
