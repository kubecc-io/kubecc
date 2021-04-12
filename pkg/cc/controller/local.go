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

	"github.com/kubecc-io/kubecc/pkg/cc"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
)

type localRunnerManager struct {
	ap *cc.ArgParser
}

func (m localRunnerManager) Process(
	ctx run.PairContext,
	request interface{},
) (interface{}, error) {
	tracer := meta.Tracer(ctx)

	span, sctx := opentracing.StartSpanFromContextWithTracer(
		ctx, tracer, "run-local")
	defer span.Finish()
	req := request.(*types.RunRequest)
	lg := meta.Log(ctx)

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	task := cc.NewCompileTask(req.GetToolchain(), m.ap,
		run.WithContext(sctx),
		run.WithLog(meta.Log(ctx)),
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
