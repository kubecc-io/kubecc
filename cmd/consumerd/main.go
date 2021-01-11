package main

import "go.uber.org/zap"

var (
	log *zap.Logger
)

func init() {
	conf := zap.NewDevelopmentConfig()
	conf.OutputPaths = []string{"/tmp/consumerd.log"}
	conf.ErrorOutputPaths = []string{"/tmp/consumerd.log"}

	lg, err := conf.Build(zap.AddStacktrace(zap.ErrorLevel))
	if err != nil {
		panic(err)
	}
	log = lg
}

func main() {
	startConsumerd()
}
