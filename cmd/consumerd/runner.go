package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func runPreprocessor(
	req *types.RunRequest,
	info *cc.ArgsInfo,
) ([]byte, error) {
	outBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	runner := run.NewPreprocessRunner(
		run.WithEnv(req.Env),
		run.WithLogger(log),
		run.WithOutputWriter(outBuf),
		run.WithStderr(stderrBuf),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)

	err := runner.Run(info)
	if err != nil {
		return nil, err
	}
	return outBuf.Bytes(), nil
}

func runRequestLocal(
	req *types.RunRequest,
	info *cc.ArgsInfo,
	executor *run.Executor,
) (*types.RunResponse, error) {
	stderrBuf := new(bytes.Buffer)

	runner := run.NewCompileRunner(
		run.InPlace(true),
		run.WithEnv(req.Env),
		run.WithLogger(log),
		run.WithStderr(stderrBuf),
		run.WithUidGid(req.UID, req.GID),
		run.WithWorkDir(req.WorkDir),
	)

	t := run.NewTask(context.Background(), runner, info)
	err := executor.Exec(t)

	if err != nil && run.IsCompilerError(err) {
		log.With(zap.Error(err)).Debug("Compiler error")
		return &types.RunResponse{
			Success: false,
			Stderr:  stderrBuf.String(),
		}, nil
	} else if err != nil {
		return nil, err
	}

	log.With(zap.Error(err)).Debug("Local run success")
	return &types.RunResponse{
		Success: true,
		Stderr:  stderrBuf.String(),
	}, nil
}

func runRequestRemote(
	ctx context.Context,
	req *types.RunRequest,
	info *cc.ArgsInfo,
	client types.SchedulerClient,
) (*types.RunResponse, error) {

	info.SubstitutePreprocessorOptions()

	var preprocessedSource []byte
	opt := info.ActionOpt()

	log.Debug("Preprocessing")
	info.SetActionOpt(cc.Preprocess)
	var err error
	preprocessedSource, err = runPreprocessor(req, info)
	if err != nil {
		return nil, err
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
	log.Debug("Starting remote compile")
	resp, err := client.Compile(ctx, &types.CompileRequest{
		Command:            "/bin/gcc", // todo
		Args:               info.Args,
		PreprocessedSource: preprocessedSource,
	}, grpc.UseCompressor(gzip.Name))
	if err != nil {
		log.With(zap.Error(err)).Debug("Remote compile failed")
		return nil, err
	}
	log.Debug("Remote compile completed")
	switch resp.CompileResult {
	case types.CompileResponse_Success:
		f, err := os.OpenFile(outputPath,
			os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0777)
		if err != nil {
			log.With(zap.Error(err)).Debug("Failed to open output file")
			return nil, err
		}
		_, err = io.Copy(f, bytes.NewReader(resp.GetCompiledSource()))
		if err != nil {
			log.With(zap.Error(err)).Debug("Copy failed")
			return nil, err
		}
		return &types.RunResponse{
			Success: true,
			Stderr:  resp.GetError(),
		}, nil
	case types.CompileResponse_Fail:
		log.Error(resp.GetError())
		return &types.RunResponse{
			Success: false,
			Stderr:  resp.GetError(),
		}, nil
	default:
		return nil, status.Error(codes.Internal, "Bad response from server")
	}
}
