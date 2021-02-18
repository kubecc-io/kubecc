package toolchain

import (
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	testtoolchain "github.com/cobalt77/kubecc/internal/testutil"
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
	lg := logkc.LogFromContext(ctx.ServerContext)
	lg.Info("=> Receiving remote")

	span, _ := opentracing.StartSpanFromContext(ctx.ClientContext, "run-recv")
	defer span.Finish()
	req := request.(*types.CompileRequest)
	ap := testutil.TestArgParser{
		Args: req.Args,
	}
	ap.Parse()
	task := run.NewTask(ctx.ClientContext, &testtoolchain.SleepRunner{
		Duration: ap.Duration,
	}, req.GetToolchain())
	err = x.Exec(task)
	if err != nil {
		panic(err)
	}
	lg.Info("=> Done.")
	return &types.CompileResponse{}, nil
}
