package main

import (
	"os"
	"path"

	"go.uber.org/zap"
)

var (
	log *zap.Logger
)

func init() {
	conf := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		OutputPaths:      []string{"/tmp/agent.log"},
		ErrorOutputPaths: []string{"/tmp/agent.log"},
	}

	lg, err := conf.Build(zap.AddStacktrace(zap.ErrorLevel))

	if err != nil {
		panic(err)
	}
	log = lg
}

func main() {
	InitConfig()
	switch path.Base(os.Args[0]) {
	case "agent", "__debug_bin":
		Execute()
	default:
		// Assume we are symlinked to a compiler
		connectOrFork()
	}
}
