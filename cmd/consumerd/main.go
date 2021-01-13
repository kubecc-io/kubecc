package main

import (
	"github.com/cobalt77/kubecc/internal/consumer"
	"github.com/cobalt77/kubecc/internal/lll"
)

func main() {
	lll.Setup("D")
	lll.PrintHeader()

	consumer.InitConfig()
	startConsumerd()
}
