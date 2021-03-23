package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/cobalt77/kubecc/pkg/kubecc"
	"github.com/cobalt77/kubecc/pkg/kubecc/tools"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	basename := filepath.Base(os.Args[0])
	for _, name := range tools.ConsumerNames {
		if basename == name {
			tools.ConsumerCmd.Run(tools.ConsumerCmd, os.Args[1:])
			return
		}
	}
	kubecc.Execute()
}
