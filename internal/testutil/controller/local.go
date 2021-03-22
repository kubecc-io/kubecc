package controller

import (
	"github.com/cobalt77/kubecc/internal/testutil"
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

	ap := testutil.SleepArgParser{
		Args: req.Args,
	}
	ap.Parse()
	t := &testutil.SleepTask{
		Duration: ap.Duration,
	}
	t.Run()
	if err := t.Err(); err != nil {
		panic(err)
	}
	lg.Info("=> Done.")
	return &types.RunResponse{
		ReturnCode: 0,
	}, nil
}
