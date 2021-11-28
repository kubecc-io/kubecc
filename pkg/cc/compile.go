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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kubecc-io/kubecc/pkg/run"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
)

type compileTask struct {
	util.NullableError
	run.TaskOptions

	tc *types.Toolchain
	ap *ArgParser
}

func NewCompileTask(tc *types.Toolchain, ap *ArgParser, opts ...run.TaskOption) run.Task {
	r := &compileTask{
		tc: tc,
		ap: ap,
	}
	r.Apply(opts...)
	return r
}

// Run the compiler with the current args.
func (t *compileTask) Run() {
	info := t.ap
	t.Log.With(
		zap.String("compiler", t.tc.Executable),
		zap.Object("info", info),
	).Debug("Received run request")

	var tmpFileName string
	if info.OutputArgIndex != -1 && !t.NoTempFile {
		// Important! the temp file's extension must match the original
		ext := filepath.Ext(info.Args[info.OutputArgIndex])
		if ext == "" && info.Args[info.OutputArgIndex] == "-" {
			ext = ".o"
		}
		topLevelDir, err := util.TopLevelTempDir()
		if err != nil {
			t.SetErr(err)
			return
		}
		tmp, err := os.CreateTemp(topLevelDir, fmt.Sprintf("kubecc_*%s", ext))
		if err != nil {
			t.Log.With(zap.Error(err)).Error("Can't create temporary files")
			t.SetErr(err)
			return
		}
		tmpFileName = tmp.Name()
		t.Log.With(
			zap.String("old", info.Args[info.OutputArgIndex]),
			zap.String("new", tmp.Name()),
		).Info("Replacing output path")
		err = info.ReplaceOutputPath(tmp.Name())
		if err != nil {
			t.Log.With(zap.Error(err)).Error("Error replacing output path")
			t.SetErr(err)
			return
		}
	}
	stderrBuf := new(bytes.Buffer)
	cmd := exec.CommandContext(t.Context, t.tc.Executable, info.Args...)
	cmd.Env = t.Env
	cmd.Dir = t.WorkDir
	cmd.Stdout = t.Stdout
	cmd.Stderr = io.MultiWriter(t.Stderr, stderrBuf)
	cmd.Stdin = t.Stdin

	t.Log.With(zap.Array("args", types.NewStringSliceEncoder(cmd.Args))).Info("Running compiler")
	if err := cmd.Start(); err != nil {
		t.SetErr(err)
		return
	}
	err := cmd.Wait()
	if err != nil {
		t.Log.With(zap.Error(err)).Error("Compiler error")
		t.SetErr(run.NewCompilerError(stderrBuf.String()))
		return
	}
	if t.OutputWriter != nil && !t.NoTempFile {
		_, err = t.OutputWriter.Write([]byte(tmpFileName))
		t.SetErr(err)
		return
	}
	t.SetErr(nil)
}
