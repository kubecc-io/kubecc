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

package cc

import (
	"bytes"
	"io"
	"os/exec"
	"syscall"

	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
)

type preprocessTask struct {
	util.NullableError
	run.TaskOptions

	tc *types.Toolchain
	ap *ArgParser
}

func NewPreprocessTask(tc *types.Toolchain, ap *ArgParser, opts ...run.TaskOption) run.Task {
	r := &preprocessTask{
		tc: tc,
		ap: ap,
	}
	r.Apply(opts...)
	return r
}

func (t *preprocessTask) Run() {
	info := t.ap
	if info.OutputArgIndex != -1 {
		t.Log.Debug("Replacing output path with '-'")
		old := info.Args[info.OutputArgIndex]
		info.ReplaceOutputPath("-")
		defer info.ReplaceOutputPath(old)
	}
	stderrBuf := new(bytes.Buffer)
	cmd := exec.CommandContext(t.Context, t.tc.Executable, info.Args...)
	cmd.Env = t.Env
	cmd.Dir = t.WorkDir
	cmd.Stdout = t.OutputWriter
	cmd.Stderr = io.MultiWriter(t.Stderr, stderrBuf)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         t.UID,
			Gid:         t.GID,
			NoSetGroups: true,
		},
	}
	err := cmd.Start()
	if err != nil {
		t.SetErr(err)
		return
	}
	ch := make(chan struct{})
	defer close(ch)
	go func() {
		select {
		case <-t.Context.Done():
			if !cmd.ProcessState.Exited() {
				if err := cmd.Process.Kill(); err != nil {
					info.lg.With(zap.Error(err)).
						Warn("Error trying to kill preprocessor")
				}
			}
		case <-ch:
		}
	}()
	err = cmd.Wait()
	if err != nil {
		t.Log.With(zap.Error(err)).Error("Compiler error")
		t.SetErr(run.NewCompilerError(stderrBuf.String()))
		return
	}
	t.SetErr(nil)
	return
}
