/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package toolchain

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/kubecc-io/kubecc/pkg/cc"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recvRemoteRunnerManager struct{}

func (m *recvRemoteRunnerManager) Process(
	ctx run.PairContext,
	request interface{},
) (interface{}, error) {
	req := request.(*types.CompileRequest)
	lg := meta.Log(ctx)
	ap := cc.NewArgParser(ctx, req.Args)
	ap.Parse()
	lg.With(zap.Object("args", ap)).Info("Compile starting")
	stderrBuf := new(bytes.Buffer)
	outputFilename := new(bytes.Buffer)

	inputFilename := ap.Args[ap.InputArgIndex]
	topLevelDir, err := util.TopLevelTempDir()
	if err != nil {
		return nil, err
	}
	tmpDir, err := os.MkdirTemp(topLevelDir, "*")
	if err != nil {
		return nil, err
	}
	tmp, err := os.Create(filepath.Join(tmpDir, filepath.Base(inputFilename)))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if _, err := io.Copy(tmp, bytes.NewReader(req.PreprocessedSource)); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	tmp.Close()
	defer os.RemoveAll(tmpDir)

	if err := ap.ReplaceInputPath(tmp.Name()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	task := cc.NewCompileTask(req.GetToolchain(), ap,
		run.WithContext(ctx),
		run.WithLog(lg),
		run.WithOutputWriter(outputFilename),
		run.WithOutputStreams(io.Discard, stderrBuf),
	)
	task.Run()

	err = task.Err()
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
	data, err := os.ReadFile(outputFilename.String())
	if err != nil {
		lg.With(zap.Error(err)).Info("Error reading temp file")
		return nil, status.Error(codes.Internal, err.Error())
	}
	err = os.Remove(outputFilename.String())
	if err != nil {
		lg.With(zap.Error(err)).Info("Error removing temp file")
	}
	if len(data) == 0 {
		return nil, status.Error(codes.Internal, "Compiled object is empty")
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
