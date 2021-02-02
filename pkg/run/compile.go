package run

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/cobalt77/kubecc/pkg/cc"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
)

type compileRunner struct {
	RunnerOptions
}

func NewCompileRunner(opts ...RunOption) Runner {
	r := &compileRunner{}
	r.Apply(opts...)
	return r
}

// Run the compiler with the current args.
func (r *compileRunner) Run(compiler string, info *cc.ArgParser) error {
	r.lg.With(
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
		tmp, err := ioutil.TempFile(
			"", fmt.Sprintf("kubecc_*%s", ext))
		if err != nil {
			r.lg.With(zap.Error(err)).Fatal("Can't create temporary files")
		}
		tmpFileName = tmp.Name()
		r.lg.With(
			zap.String("old", info.Args[info.OutputArgIndex]),
			zap.String("new", tmp.Name()),
		).Info("Replacing output path")
		err = info.ReplaceOutputPath(tmp.Name())
		if err != nil {
			r.lg.With(zap.Error(err)).Error("Error replacing output path")
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

	r.lg.With(zap.Array("args", types.NewStringSliceEncoder(cmd.Args))).Info("Running compiler")
	if err := cmd.Start(); err != nil {
		return err
	}
	err := cmd.Wait()
	if err != nil {
		r.lg.With(zap.Error(err)).Error("Compiler error")
		return NewCompilerError(stderrBuf.String())
	}
	if r.OutputWriter != nil && !r.NoTempFile {
		_, err = r.OutputWriter.Write([]byte(tmpFileName))
		return err
	}
	return nil
}
