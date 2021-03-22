package toolchain

import (
	"bytes"
	"io"
	"os"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recvRemoteRunnerManager struct {
	tc *types.Toolchain
}

func (m *recvRemoteRunnerManager) Process(
	ctx run.Contexts,
	request interface{},
) (interface{}, error) {
	req := request.(*types.CompileRequest)
	lg := meta.Log(ctx.ServerContext)
	ap := cc.NewArgParser(ctx.ServerContext, req.Args)
	ap.Parse()
	lg.With(zap.Object("args", ap)).Info("Compile starting")

	stderrBuf := new(bytes.Buffer)
	tmpFilename := new(bytes.Buffer)
	task := cc.NewCompileTask(m.tc, ap,
		run.WithContext(ctx.ClientContext),
		run.WithLog(meta.Log(ctx.ServerContext)),
		run.WithOutputWriter(tmpFilename),
		run.WithOutputStreams(io.Discard, stderrBuf),
		run.WithStdin(bytes.NewReader(req.GetPreprocessedSource())),
	)
	task.Run()

	err := task.Err()
	lg.With(zap.Error(err)).Info("Compile finished")
	if err != nil && run.IsCompilerError(err) {
		return &types.CompileResponse{
			RequestID:     req.GetRequestID(),
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
		RequestID:     req.GetRequestID(),
		CompileResult: types.CompileResponse_Success,
		Data: &types.CompileResponse_CompiledSource{
			CompiledSource: data,
		},
	}, nil
}
