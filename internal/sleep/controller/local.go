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
	t := run.NewExecCommandTask(req.GetToolchain(),
		run.WithArgs(req.Args),
		run.WithEnv(req.Env),
		run.WithWorkDir(req.WorkDir),
		run.WithOutputStreams(os.Stdout, os.Stderr),
		run.WithContext(ctx.ClientContext),
	)
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
