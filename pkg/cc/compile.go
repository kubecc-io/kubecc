package cc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
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
		tmp, err := os.CreateTemp(
			"", fmt.Sprintf("kubecc_*%s", ext))
		if err != nil {
			t.Log.With(zap.Error(err)).Fatal("Can't create temporary files")
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
