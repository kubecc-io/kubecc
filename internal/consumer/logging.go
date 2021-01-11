package consumer

import "go.uber.org/zap"

var (
	log *zap.Logger
)

func init() {
	conf := zap.NewDevelopmentConfig()
	conf.OutputPaths = []string{"stdout", "/tmp/agent.log"}
	conf.ErrorOutputPaths = []string{"stderr", "/tmp/agent.log"}

	lg, err := conf.Build(zap.AddStacktrace(zap.ErrorLevel))
	if err != nil {
		panic(err)
	}
	log = lg
}
