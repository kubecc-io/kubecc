package consumerd

import (
	"bytes"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
)

type localToolchainRunner struct {
	run.ToolchainRunner
	ArgParser *cc.ArgParser
}

func (r localToolchainRunner) Run(
	ctx run.Contexts,
	executor run.Executor,
	request interface{},
) (interface{}, error) {
	span, sctx := opentracing.StartSpanFromContext(ctx.ClientContext, "run-local")
	defer span.Finish()
	req := request.(*types.RunRequest)
	lg := logkc.LogFromContext(ctx.ServerContext)
	ap := r.ArgParser

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	runner := cc.NewCompileRunner(ap,
		run.WithContext(logkc.ContextWithLog(ctx.ClientContext, lg)),
		run.InPlace(true),
		run.WithEnv(req.Env),
		run.WithOutputStreams(stdoutBuf, stderrBuf),
		run.WithStdin(bytes.NewReader(req.Stdin)),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)

	t := run.NewTask(sctx, runner, req.GetToolchain())
	err := executor.Exec(t)

	if err != nil && run.IsCompilerError(err) {
		lg.With(zap.Error(err), zap.Object("args", ap)).Error("Compiler error")
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
