package toolchain

import (
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
)

type remoteCompileRunner struct {
	run.RunnerOptions

	client types.SchedulerClient
}

func NewRemoteCompileRunner(
	client types.SchedulerClient,
	opts ...run.RunOption,
) run.Runner {
	r := &remoteCompileRunner{
		client: client,
	}
	r.Apply(opts...)
	return r
}

func (r *remoteCompileRunner) Run(ctx context.Context, tc *types.Toolchain) error {
	preprocessedSource, err := io.ReadAll(r.Stdin)
	if err != nil {
		return err
	}
	resp, err := r.client.Compile(ctx, &types.CompileRequest{
		Toolchain:          tc,
		Args:               r.Args,
		PreprocessedSource: preprocessedSource,
	}, grpc.UseCompressor(gzip.Name))
	if r.OutputVar != nil {
		r.OutputVar = resp
	}
	return err
}

type sendRemoteRunnerManager struct {
	schedulerClient types.SchedulerClient
	ArgParser       *cc.ArgParser
}

func runPreprocessor(
	ctx context.Context,
	ap *cc.ArgParser,
	req *types.RunRequest,
) ([]byte, *types.RunResponse) {
	tracer := meta.Tracer(ctx)
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx, tracer, "preprocess")
	defer span.Finish()
	lg := meta.Log(ctx)

	outBuf := new(bytes.Buffer)
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	runner := cc.NewPreprocessRunner(ap,
		run.WithContext(sctx),
		run.WithLog(lg),
		run.WithEnv(req.Env),
		run.WithOutputWriter(outBuf),
		run.WithOutputStreams(stdoutBuf, stderrBuf),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)

	if err := runner.Run(sctx, req.GetToolchain()); err != nil {
		stderr := stderrBuf.Bytes()
		lg.With(
			zap.Error(err),
			zap.Object("args", ap),
			zap.ByteString("stderr", stderr),
		).Error("Compiler error")
		return nil, &types.RunResponse{
			ReturnCode: 1,
			Stdout:     stdoutBuf.Bytes(),
			Stderr:     stderr,
		}
	}
	return outBuf.Bytes(), nil
}

func (r sendRemoteRunnerManager) Run(
	ctx run.Contexts,
	executor run.Executor,
	request interface{},
) (interface{}, error) {
	tracer := ctx.ServerContext.Tracer()
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-remote")
	defer span.Finish()
	req := request.(*types.RunRequest)
	lg := ctx.ServerContext.Log()
	ap := r.ArgParser

	ap.ConfigurePreprocessorOptions()

	opt := ap.ActionOpt()

	lg.Debug("Preprocessing")
	ap.SetActionOpt(cc.Preprocess)
	preprocessedSource, errResp := runPreprocessor(sctx, ap, req)
	if errResp != nil {
		return errResp, nil
	}
	ap.SetActionOpt(opt)

	ap.ReplaceInputPath("-") // Read from stdin

	var outputPath string
	if ap.OutputArgIndex >= 0 && ap.OutputArgIndex < len(ap.Args) {
		outputPath = ap.Args[ap.OutputArgIndex]
	} else {
		return nil, status.Error(codes.InvalidArgument, "No output path given")
	}
	if !filepath.IsAbs(outputPath) {
		outputPath = path.Join(req.WorkDir, outputPath)
	}

	// Compile remote
	ap.RemoveLocalArgs()
	lg.Debug("Starting remote compile")
	resp := &types.CompileResponse{}
	runner := NewRemoteCompileRunner(r.schedulerClient,
		run.WithArgs(ap.Args),
		run.WithStdin(bytes.NewReader(preprocessedSource)),
		run.WithOutputVar(resp),
	)
	task := run.Begin(sctx, runner, req.GetToolchain())
	err := executor.Exec(task)

	if err != nil {
		lg.With(zap.Error(err)).Debug("Remote compile failed")
		return nil, err
	}
	lg.Debug("Remote compile completed")
	switch resp.CompileResult {
	case types.CompileResponse_Success:
		f, err := os.OpenFile(outputPath,
			os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0777)
		if err != nil {
			lg.With(zap.Error(err)).Debug("Failed to open output file")
			return nil, err
		}
		_, err = io.Copy(f, bytes.NewReader(resp.GetCompiledSource()))
		if err != nil {
			lg.With(zap.Error(err)).Debug("Copy failed")
			return nil, err
		}
		return &types.RunResponse{
			ReturnCode: 0,
			Stdout:     []byte{},
			Stderr:     []byte(resp.GetError()),
		}, nil
	case types.CompileResponse_Fail:
		err := tools.AnalyzeErrors(resp.GetError())
		if err != nil {
			return nil, status.Error(codes.Internal, "Internal error")
		}
		return &types.RunResponse{
			ReturnCode: 1,
			Stdout:     []byte{},
			Stderr:     []byte(resp.GetError()),
		}, nil
	default:
		return nil, status.Error(codes.Internal, "Bad response from server")
	}
}
