package controller

import (
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type recvRemoteRunnerManager struct {
}

func (m recvRemoteRunnerManager) Process(
	ctx run.Contexts,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)

	lg.Info("=> Receiving remote")
	req := request.(*types.CompileRequest)
	t := run.NewExecCommandTask(req.GetToolchain(),
		run.WithArgs(req.Args),
		run.WithContext(ctx.ClientContext),
	)
	t.Run()
	if err := t.Err(); err != nil {
		lg.Error(err)
		return &types.CompileResponse{
			RequestID:     req.RequestID,
			CompileResult: types.CompileResponse_Fail,
			Data: &types.CompileResponse_Error{
				Error: err.Error(),
			},
		}, err
	}
	lg.Info("=> Done.")
	return &types.CompileResponse{
		RequestID:     req.RequestID,
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: []byte{},
		},
	}, nil
}
