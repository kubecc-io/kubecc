package toolchain

import (
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
)

type sendRemoteRunnerManager struct {
	client types.SchedulerClient
}

func (m sendRemoteRunnerManager) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	span, _ := opentracing.StartSpanFromContext(ctx.ClientContext, "run-send")
	defer span.Finish()
	req := request.(*types.RunRequest)

	_, err = m.client.Compile(ctx.ClientContext, &types.CompileRequest{
		Toolchain: req.GetToolchain(),
		Args:      req.Args,
	})
	if err != nil {
		panic(err)
	}
	return &types.RunResponse{
		ReturnCode: 0,
	}, nil
}
