package toolchain

import (
	"flag"
	"time"

	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
)

type recvRemoteRunnerManager struct {
}

func (m recvRemoteRunnerManager) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	span, _ := opentracing.StartSpanFromContext(ctx.ClientContext, "run-recv")
	defer span.Finish()
	req := request.(*types.CompileRequest)

	var duration string
	set := flag.NewFlagSet("test", flag.PanicOnError)
	set.StringVar(&duration, "duration", "1s", "")
	set.Parse(req.Args)

	d, err := time.ParseDuration(duration)
	if err != nil {
		panic(err)
	}
	time.Sleep(d)
	return &types.CompileResponse{}, nil
}
