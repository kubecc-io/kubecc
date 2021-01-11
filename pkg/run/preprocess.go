package run

import (
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/cobalt77/kubecc/pkg/cc"
	"go.uber.org/zap"
)

type preprocessRunner struct {
	RunnerOptions
}

func NewPreprocessRunner(opts ...RunOption) Runner {
	r := &preprocessRunner{}
	r.Apply(opts...)
	return r
}

func (r *preprocessRunner) Run(info *cc.ArgsInfo) error {
	log := r.Logger

	if info.OutputArgIndex != -1 {
		log.Debug("Replacing output path with '-'")
		info.ReplaceOutputPath("-")
	}

	gcc, _ := filepath.EvalSymlinks("/bin/gcc")
	cmd := exec.Command(gcc, info.Args...) // todo
	cmd.Env = r.Env
	cmd.Dir = r.WorkDir
	cmd.Stdout = r.OutputWriter
	cmd.Stderr = r.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         r.UID,
			Gid:         r.GID,
			NoSetGroups: true,
		},
	}
	err := cmd.Run()
	if err != nil {
		log.With(zap.Error(err)).Error("Compiler error")
		return NewCompilerError(err.Error())
	}
	return nil
}
