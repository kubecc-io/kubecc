package toolchain

import (
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type remoteCompileTask struct {
	util.NullableError
	run.TaskOptions

	tc     *types.Toolchain
	client *clients.CompileRequestClient
}

func makeRemoteCompileTask(
	client *clients.CompileRequestClient,
	tc *types.Toolchain,
	opts ...run.TaskOption,
) run.Task {
	m := &remoteCompileTask{
		tc:     tc,
		client: client,
	}
	m.Apply(opts...)
	return m
}

func (m *remoteCompileTask) Run() {
	preprocessedSource, err := io.ReadAll(m.Stdin)
	if err != nil {
		m.SetErr(err)
		return
	}
	resp, err := m.client.Compile(&types.CompileRequest{
		RequestID:          uuid.NewString(),
		Toolchain:          m.tc,
		Args:               m.Args,
		PreprocessedSource: preprocessedSource,
	})
	if m.OutputVar != nil {
		m.OutputVar = resp
	}
	m.SetErr(nil)
}

type sendRemoteRunnerManager struct {
	tc        *types.Toolchain
	reqClient *clients.CompileRequestClient
	ap        *cc.ArgParser
}

func runPreprocessor(
	ctx context.Context,
	tc *types.Toolchain,
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

	task := cc.NewPreprocessTask(tc, ap,
		run.WithContext(sctx),
		run.WithLog(lg),
		run.WithEnv(req.Env),
		run.WithOutputWriter(outBuf),
		run.WithOutputStreams(stdoutBuf, stderrBuf),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)

	if err := run.RunWait(task); err != nil {
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

func (m sendRemoteRunnerManager) Process(
	ctx run.Contexts,
	request interface{},
) (interface{}, error) {
	tracer := meta.Tracer(ctx.ServerContext)
	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx.ClientContext, tracer, "run-remote")
	defer span.Finish()
	req := request.(*types.RunRequest)
	lg := meta.Log(ctx.ServerContext)
	ap := m.ap

	ap.ConfigurePreprocessorOptions()

	opt := ap.ActionOpt()

	lg.Debug("Preprocessing")
	ap.SetActionOpt(cc.Preprocess)
	preprocessedSource, errResp := runPreprocessor(sctx, m.tc, ap, req)
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
	task := makeRemoteCompileTask(m.reqClient, req.GetToolchain(),
		run.WithContext(sctx),
		run.WithArgs(ap.Args),
		run.WithStdin(bytes.NewReader(preprocessedSource)),
		run.WithOutputVar(resp),
	)
	task.Run()

	if err := task.Err(); err != nil {
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
		err := util.AnalyzeErrors(resp.GetError())
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
