package toolchain

import (
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/tracing"
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
	lg := logkc.LogFromContext(ctx.ServerContext)
	tracer := tracing.TracerFromContext(ctx.ServerContext)

	lg.Info("Sending remote")
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-send")
	defer span.Finish()
	req := request.(*types.RunRequest)

	_, err = m.client.Compile(sctx, &types.CompileRequest{
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