package controller

import (
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
)

type recvRemoteRunnerManager struct{}

func (m recvRemoteRunnerManager) Process(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)

	lg.Info("=> Receiving remote")
	req := request.(*types.CompileRequest)
	ap := testutil.SleepArgParser{
		Args: req.Args,
	}
	ap.Parse()
	err = x.Exec(&testutil.SleepTask{
		Duration: ap.Duration,
	})
	if err != nil {
		panic(err)
	}
	lg.Info("=> Done.")
	return &types.CompileResponse{
		RequestID:     req.RequestID,
		CompileResult: types.CompileResponse_Success,
	}, nil
}

type recvRemoteRunnerManagerSim struct{}

func (m recvRemoteRunnerManagerSim) Process(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	panic("Unimplemented")
}
