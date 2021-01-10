package main

import (
	"os"

	"go.uber.org/zap"
)

var (
	log *zap.Logger
)

func main() {
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
	initConfig()
	if len(os.Args) == 2 && os.Args[1] == "--become-leader" {
		runLeader()
	} else {
		runConsumer()
	}
}
