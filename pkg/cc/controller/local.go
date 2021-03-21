package toolchain

import (
	"bytes"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
)

type localRunnerManager struct {
	tc *types.Toolchain
	ap *cc.ArgParser
}

func (m localRunnerManager) Process(
	ctx run.Contexts,
	request interface{},
) (interface{}, error) {
	tracer := meta.Tracer(ctx.ServerContext)

	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-local")
	defer span.Finish()
	req := request.(*types.RunRequest)
	lg := meta.Log(ctx.ServerContext)

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	task := cc.NewCompileTask(m.tc, m.ap,
		run.WithContext(sctx),
		run.WithLog(meta.Log(ctx.ServerContext)),
		run.InPlace(true),
		run.WithEnv(req.Env),
		run.WithOutputStreams(stdoutBuf, stderrBuf),
		run.WithStdin(bytes.NewReader(req.Stdin)),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)
	task.Run()

	err := task.Err()
	if err != nil && run.IsCompilerError(err) {
		lg.With(zap.Error(err), zap.Object("args", m.ap)).Error("Compiler error")
		errString := stderrBuf.String()
		lg.Error(errString)
		return &types.RunResponse{
			ReturnCode: 1,
			Stdout:     stdoutBuf.Bytes(),
			Stderr:     stderrBuf.Bytes(),
		}, nil
	} else if err != nil {
		return nil, err
	}

	lg.With(zap.Error(err)).Debug("Local run success")
	return &types.RunResponse{
		ReturnCode: 0,
		Stdout:     stdoutBuf.Bytes(),
		Stderr:     stderrBuf.Bytes(),
	}, nil
}
