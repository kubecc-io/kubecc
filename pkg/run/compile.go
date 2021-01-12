package run

import (
	"bytes"
	"io"
	"io/ioutil"
	"os/exec"

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
func (r *compileRunner) Run(info *cc.ArgsInfo) error {
	log := r.Logger
	log.With(zap.Object("info", info)).Debug("Received run request")

	var tmpFileName string
	if info.OutputArgIndex != -1 && !r.NoTempFile {
		tmp, err := ioutil.TempFile("", "kubecc")
		if err != nil {
			log.With(zap.Error(err)).Fatal("Can't create temporary files")
		}
		tmpFileName = tmp.Name()
		log.With(zap.String("newPath", tmp.Name())).Info("Replacing output path")
		err = info.ReplaceOutputPath(tmp.Name())
		if err != nil {
			log.With(zap.Error(err)).Error("Error replacing output path")
			return err
		}
	}
	stderrBuf := new(bytes.Buffer)
	cmd := exec.Command("/bin/gcc", info.Args...) // todo
	cmd.Env = r.Env
	cmd.Dir = r.WorkDir
	cmd.Stdout = nil
	cmd.Stderr = io.MultiWriter(r.Stderr, stderrBuf)
	cmd.Stdin = r.Stdin

	log.With(zap.Array("args", types.NewStringSliceEncoder(cmd.Args))).Info("Running compiler")
	if err := cmd.Start(); err != nil {
		return err
	}
	err := cmd.Wait()
	if err != nil {
		log.With(zap.Error(err)).Error("Compiler error")
		return NewCompilerError(stderrBuf.String())
	}
	if r.OutputWriter != nil && !r.NoTempFile {
		_, err = r.OutputWriter.Write([]byte(tmpFileName))
		return err
	}
	return nil
}
