package toolchain

import (
	"os"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
)

type localRunnerManager struct{}

func (m localRunnerManager) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)
	tracer := meta.Tracer(ctx.ServerContext)
	lg.Info("=> Running local")
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-local")
	defer span.Finish()
	req := request.(*types.RunRequest)
	task := run.Begin(sctx,
		&run.BasicExecutableRunner{
			Args:    req.Args,
			Env:     req.Env,
			WorkDir: req.WorkDir,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		}, req.GetToolchain())
	err = x.Exec(task)
	if err != nil {
		lg.Error(err)
		return &types.RunResponse{
			ReturnCode: 1,
		}, err
	}
	lg.Info("=> Done.")
	return &types.RunResponse{
		ReturnCode: 0,
	}, nil
}
