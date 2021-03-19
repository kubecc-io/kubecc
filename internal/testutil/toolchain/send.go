package toolchain

import (
	"errors"

	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type sendRemoteRunnerManager struct {
	client *clients.CompileRequestClient
}

func (m sendRemoteRunnerManager) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)
	tracer := meta.Tracer(ctx.ServerContext)

	lg.Info("Sending remote")
	span, _ := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-send")
	defer span.Finish()
	req := request.(*types.RunRequest)
	id := uuid.NewString()
	_, err = m.client.Compile(&types.CompileRequest{
		RequestID: id,
		Toolchain: req.GetToolchain(),
		Args:      req.Args,
	})
	if err != nil {
		if errors.Is(err, clients.ErrStreamNotReady) {
			return nil, err
		}
		panic(err)
	}
	return &types.RunResponse{
		ReturnCode: 0,
	}, nil
}

type sendRemoteRunnerManagerNoop struct{}

func (m sendRemoteRunnerManagerNoop) Run(
	ctx run.Contexts,
	x run.Executor,
	request interface{},
) (response interface{}, err error) {
	lg := meta.Log(ctx.ServerContext)
	lg.Info("=> Sending remote")
	req := request.(*types.RunRequest)
	task := run.Begin(ctx.ClientContext, &testutil.NoopRunner{}, req.GetToolchain())
	err = x.Exec(task)
	if err != nil {
		if errors.Is(err, clients.ErrStreamNotReady) {
			return nil, err
		}
		panic(err)
	}
	return &types.RunResponse{
		ReturnCode: 0,
	}, nil
}
