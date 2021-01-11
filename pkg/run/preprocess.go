package run

import (
	"compress/gzip"
	"os/exec"
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

	cmd := exec.Command("/bin/gcc", info.Args...) // todo
	cmd.Env = r.Env
	cmd.Dir = r.WorkDir
	if r.Compress {
		cmd.Stdout = r.OutputWriter
	} else {
		cmd.Stdout = gzip.NewWriter(r.OutputWriter)
	}
	cmd.Stderr = r.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: r.UID,
			Gid: r.GID,
		},
	}
	err := cmd.Run()
	if err != nil {
		log.With(zap.Error(err)).Error("Compiler error")
		return NewCompilerError(err.Error())
	}
	return nil
}
