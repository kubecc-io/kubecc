package run

import (
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/cobalt77/kubecc/pkg/cc"
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

	log.With(zap.Object("info", info)).Debug("Running compiler")

	tmp, err := ioutil.TempFile("", "kubecc")
	defer os.Remove(tmp.Name())
	if err != nil {
		log.With(zap.Error(err)).Fatal("Can't create temporary files")
	}
	if info.OutputArgIndex != -1 {
		log.Debug("Replacing output path")
		err = info.ReplaceOutputPath(tmp.Name())
		if err != nil {
			log.With(zap.Error(err)).Error("Error replacing output path")
			return err
		}
	}
	cmd := exec.Command("/bin/gcc", info.Args...) // todo
	cmd.Env = r.Env
	cmd.Dir = r.WorkDir
	cmd.Stdout = nil
	cmd.Stderr = r.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		log.With(zap.Error(err)).Error("Compiler error")
		return NewCompilerError(err.Error())
	}
	_, err = r.OutputWriter.Write([]byte(tmp.Name()))
	return err
}
