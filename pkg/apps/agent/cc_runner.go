package agent

import (
	"bytes"
	"io"
	"os"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CCRunner struct{}

func (r *CCRunner) Run(
	ctx run.Contexts,
	executor run.Executor,
	request interface{},
) (interface{}, error) {
	span, sctx := opentracing.StartSpanFromContext(ctx.ClientContext, "queue")
	defer span.Finish()

	req := request.(*types.CompileRequest)
	lg := logkc.LogFromContext(ctx.ServerContext)
	ap := cc.NewArgParser(ctx.ServerContext, req.Args)
	ap.Parse()
	lg.With(zap.Object("args", ap)).Info("Compile starting")

	stderrBuf := new(bytes.Buffer)
	tmpFilename := new(bytes.Buffer)
	runner := cc.NewCompileRunner(ap,
		run.WithContext(logkc.ContextWithLog(ctx.ClientContext, lg)),
		run.WithOutputWriter(tmpFilename),
		run.WithOutputStreams(io.Discard, stderrBuf),
		run.WithStdin(bytes.NewReader(req.GetPreprocessedSource())),
	)
	task := run.NewTask(sctx, runner, req.Toolchain)
	err := executor.Exec(task)
	lg.With(zap.Error(err)).Info("Compile finished")
	if err != nil && run.IsCompilerError(err) {
		return &types.CompileResponse{
			CompileResult: types.CompileResponse_Fail,
			Data: &types.CompileResponse_Error{
				Error: stderrBuf.String(),
			},
		}, nil
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	data, err := os.ReadFile(tmpFilename.String())
	if err != nil {
		lg.With(zap.Error(err)).Info("Error reading temp file")
		return nil, status.Error(codes.Internal, err.Error())
	}
	err = os.Remove(tmpFilename.String())
	if err != nil {
		lg.With(zap.Error(err)).Info("Error removing temp file")
	}
	lg.With(zap.Error(err)).Info("Sending results")
	return &types.CompileResponse{
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: data,
		},
	}, nil
}
