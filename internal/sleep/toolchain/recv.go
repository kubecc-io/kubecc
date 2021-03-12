package toolchain

import (
	"github.com/cobalt77/kubecc/pkg/meta"
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
	lg := meta.Log(ctx.ServerContext)
	tracer := meta.Tracer(ctx.ServerContext)

	lg.Info("=> Receiving remote")
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-recv")
	defer span.Finish()
	req := request.(*types.CompileRequest)
	task := run.Begin(sctx,
		&run.BasicExecutableRunner{
			Args: req.Args,
		}, req.GetToolchain())
	err = x.Exec(task)
	if err != nil {
		lg.Error(err)
		return &types.CompileResponse{
			CompileResult: types.CompileResponse_Fail,
			Data: &types.CompileResponse_Error{
				Error: err.Error(),
			},
		}, err
	}
	lg.Info("=> Done.")
	return &types.CompileResponse{
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: []byte{},
		},
	}, nil
}
