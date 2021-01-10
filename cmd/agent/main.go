package main

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.InfoLevel)
	InitConfig()
	switch path.Base(os.Args[0]) {
	case "agent", "__debug_bin":
		Execute()
	default:
		// Assume we are symlinked to a compiler
		connectOrFork()
	}
}
