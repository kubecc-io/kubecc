package run

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/cobalt77/kubecc/internal/lll"
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
	if info.OutputArgIndex != -1 {
		lll.Debug("Replacing output path with '-'")
		old := info.Args[info.OutputArgIndex]
		info.ReplaceOutputPath("-")
		defer info.ReplaceOutputPath(old)
	}
	stderrBuf := new(bytes.Buffer)
	gcc, _ := filepath.EvalSymlinks("/bin/gcc")
	cmd := exec.Command(gcc, info.Args...) // todo
	cmd.Env = r.Env
	cmd.Dir = r.WorkDir
	cmd.Stdout = r.OutputWriter
	cmd.Stderr = io.MultiWriter(r.Stderr, stderrBuf)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         r.UID,
			Gid:         r.GID,
			NoSetGroups: true,
		},
	}
	err := cmd.Run()
	if err != nil {
		lll.With(zap.Error(err)).Error("Compiler error")
		return NewCompilerError(stderrBuf.String())
	}
	return nil
}
