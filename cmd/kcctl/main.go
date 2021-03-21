package main

import (
	"math/rand"
	"time"

	"github.com/cobalt77/kubecc/cmd/kcctl/commands"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	commands.Execute()
}
