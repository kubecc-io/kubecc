package toolchain

import (
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
)

type localRunnerManager struct{}

func (m localRunnerManager) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	lg := logkc.LogFromContext(ctx.ServerContext)
	tracer := tracing.TracerFromContext(ctx.ServerContext)
	lg.Info("=> Running local")
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-local")
	defer span.Finish()
	req := request.(*types.RunRequest)

	ap := testutil.TestArgParser{
		Args: req.Args,
	}
	ap.Parse()
	task := run.Begin(
		tracing.ContextWithTracer(sctx, tracer),
		&testtoolchain.SleepRunner{
			Duration: ap.Duration,
		}, req.GetToolchain())
	err = x.Exec(task)
	if err != nil {
		panic(err)
	}
	lg.Info("=> Done.")
	return &types.RunResponse{
		ReturnCode: 0,
	}, nil
}
