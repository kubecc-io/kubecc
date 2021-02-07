package consumerd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func (c *consumerd) runPreprocessor(
	ctx context.Context,
	req *types.RunRequest,
	info *cc.ArgParser,
) ([]byte, *types.RunResponse) {
	span, _ := opentracing.StartSpanFromContext(ctx, "preprocess")
	defer span.Finish()

	outBuf := new(bytes.Buffer)
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	runner := run.NewPreprocessRunner(
		run.WithContext(logkc.ContextWithLog(ctx, c.lg)),
		run.WithEnv(req.Env),
		run.WithOutputWriter(outBuf),
		run.WithOutputStreams(stdoutBuf, stderrBuf),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)

	if err := runner.Run(fmt.Sprintf("/usr/bin/%s", req.Compiler), info); err != nil {
		stderr := stderrBuf.Bytes()
		c.lg.With(
			zap.Error(err),
			zap.Object("info", info),
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

func (c *consumerd) runRequestLocal(
	ctx context.Context,
	req *types.RunRequest,
	info *cc.ArgParser,
	executor *run.Executor,
) (*types.RunResponse, error) {
	span, sctx := opentracing.StartSpanFromContext(ctx, "run-local")
	defer span.Finish()

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	runner := run.NewCompileRunner(
		run.InPlace(true),
		run.WithEnv(req.Env),
		run.WithOutputStreams(stdoutBuf, stderrBuf),
		run.WithStdin(bytes.NewReader(req.Stdin)),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)

	t := run.NewTask(sctx, runner, fmt.Sprintf("/usr/bin/%s", req.Compiler), info)
	err := executor.Exec(t)

	if err != nil && run.IsCompilerError(err) {
		c.lg.With(zap.Error(err), zap.Object("info", info)).Error("Compiler error")
		errString := stderrBuf.String()
		c.lg.Error(errString)
		return &types.RunResponse{
			ReturnCode: 1,
			Stdout:     stdoutBuf.Bytes(),
			Stderr:     stderrBuf.Bytes(),
		}, nil
	} else if err != nil {
		return nil, err
	}

	c.lg.With(zap.Error(err)).Debug("Local run success")
	return &types.RunResponse{
		ReturnCode: 0,
		Stdout:     stdoutBuf.Bytes(),
		Stderr:     stderrBuf.Bytes(),
	}, nil
}

func (c *consumerd) runRequestRemote(
	ctx context.Context,
	req *types.RunRequest,
	info *cc.ArgParser,
	client types.SchedulerClient,
) (*types.RunResponse, error) {
	span, sctx := opentracing.StartSpanFromContext(ctx, "run-remote")
	defer span.Finish()

	info.ConfigurePreprocessorOptions()

	opt := info.ActionOpt()

	c.lg.Debug("Preprocessing")
	info.SetActionOpt(cc.Preprocess)
	preprocessedSource, errResp := c.runPreprocessor(sctx, req, info)
	if errResp != nil {
		return errResp, nil
	}
	info.SetActionOpt(opt)

	info.ReplaceInputPath("-") // Read from stdin

	var outputPath string
	if info.OutputArgIndex >= 0 && info.OutputArgIndex < len(info.Args) {
		outputPath = info.Args[info.OutputArgIndex]
	} else {
		return nil, status.Error(codes.InvalidArgument, "No output path given")
	}
	if !filepath.IsAbs(outputPath) {
		outputPath = path.Join(req.WorkDir, outputPath)
	}

	// Compile remote
	info.RemoveLocalArgs()
	c.lg.Debug("Starting remote compile")
	resp, err := client.Compile(ctx, &types.CompileRequest{
		Command:            req.Compiler, // todo
		Args:               info.Args,
		PreprocessedSource: preprocessedSource,
	}, grpc.UseCompressor(gzip.Name))
	if err != nil {
		c.lg.With(zap.Error(err)).Debug("Remote compile failed")
		return nil, err
	}
	c.lg.Debug("Remote compile completed")
	switch resp.CompileResult {
	case types.CompileResponse_Success:
		f, err := os.OpenFile(outputPath,
			os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0777)
		if err != nil {
			c.lg.With(zap.Error(err)).Debug("Failed to open output file")
			return nil, err
		}
		_, err = io.Copy(f, bytes.NewReader(resp.GetCompiledSource()))
		if err != nil {
			c.lg.With(zap.Error(err)).Debug("Copy failed")
			return nil, err
		}
		return &types.RunResponse{
			ReturnCode: 0,
			Stdout:     []byte{},
			Stderr:     []byte(resp.GetError()),
		}, nil
	case types.CompileResponse_Fail:
		err := analyzeErrors(resp.GetError())
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
