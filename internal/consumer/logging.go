package consumer

import "go.uber.org/zap"

var (
	log *zap.Logger
)

func init() {
	conf := zap.NewDevelopmentConfig()
	conf.OutputPaths = []string{"/tmp/agent.log"}
	conf.ErrorOutputPaths = []string{"/tmp/agent.log"}

	lg, err := conf.Build(zap.AddStacktrace(zap.ErrorLevel))
	if err != nil {
		panic(err)
	}
	log = lg
}
