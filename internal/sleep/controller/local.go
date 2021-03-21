package controller

import (
	"os"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type localRunnerManager struct{}

func (m localRunnerManager) Process(
	ctx run.Contexts,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)
	lg.Info("=> Running local")
	req := request.(*types.RunRequest)
	t := &run.ExecCommandTask{
		Toolchain: req.GetToolchain(),
		Args:      req.Args,
		Env:       req.Env,
		WorkDir:   req.WorkDir,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	}
	t.Run()
	if err := t.Err(); err != nil {
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
