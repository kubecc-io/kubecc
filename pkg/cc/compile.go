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
	"go.uber.org/zap"
)

type compileRunner struct {
	run.RunnerOptions

	info *ArgParser
}

func NewCompileRunner(info *ArgParser, opts ...run.RunOption) run.Runner {
	r := &compileRunner{
		info: info,
	}
	r.Apply(opts...)
	return r
}

// Run the compiler with the current args.
func (r *compileRunner) Run(compiler string) error {
	info := r.info
	r.Log.With(
		zap.String("compiler", compiler),
		zap.Object("info", info),
	).Debug("Received run request")

	var tmpFileName string
	if info.OutputArgIndex != -1 && !r.NoTempFile {
		// Important! the temp file's extension must match the original
		ext := filepath.Ext(info.Args[info.OutputArgIndex])
		if ext == "" && info.Args[info.OutputArgIndex] == "-" {
			ext = ".o"
		}
		tmp, err := os.CreateTemp(
			"", fmt.Sprintf("kubecc_*%s", ext))
		if err != nil {
			r.Log.With(zap.Error(err)).Fatal("Can't create temporary files")
		}
		tmpFileName = tmp.Name()
		r.Log.With(
			zap.String("old", info.Args[info.OutputArgIndex]),
			zap.String("new", tmp.Name()),
		).Info("Replacing output path")
		err = info.ReplaceOutputPath(tmp.Name())
		if err != nil {
			r.Log.With(zap.Error(err)).Error("Error replacing output path")
			return err
		}
	}
	stderrBuf := new(bytes.Buffer)
	cmd := exec.Command(compiler, info.Args...) // todo
	cmd.Env = r.Env
	cmd.Dir = r.WorkDir
	cmd.Stdout = r.Stdout
	cmd.Stderr = io.MultiWriter(r.Stderr, stderrBuf)
	cmd.Stdin = r.Stdin

	r.Log.With(zap.Array("args", types.NewStringSliceEncoder(cmd.Args))).Info("Running compiler")
	if err := cmd.Start(); err != nil {
		return err
	}
	err := cmd.Wait()
	if err != nil {
		r.Log.With(zap.Error(err)).Error("Compiler error")
		return run.NewCompilerError(stderrBuf.String())
	}
	if r.OutputWriter != nil && !r.NoTempFile {
		_, err = r.OutputWriter.Write([]byte(tmpFileName))
		return err
	}
	return nil
}
