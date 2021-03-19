package toolchain

import (
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
)

type recvRemoteRunnerManager struct{}

func (m recvRemoteRunnerManager) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)
	tracer := meta.Tracer(ctx.ServerContext)

	lg.Info("=> Receiving remote")
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-recv")
	defer span.Finish()
	req := request.(*types.CompileRequest)
	ap := testutil.TestArgParser{
		Args: req.Args,
	}
	ap.Parse()
	task := run.Begin(sctx,
		&testutil.SleepRunner{
			Duration: ap.Duration,
		}, req.GetToolchain())
	err = x.Exec(task)
	if err != nil {
		panic(err)
	}
	lg.Info("=> Done.")
	return &types.CompileResponse{
		RequestID:     req.RequestID,
		CompileResult: types.CompileResponse_Success,
	}, nil
}

type recvRemoteRunnerManagerNoop struct{}

func (m recvRemoteRunnerManagerNoop) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	panic("Unimplemented")
}
