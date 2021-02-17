package cc

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"syscall"

	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
)

type preprocessRunner struct {
	run.RunnerOptions

	info *ArgParser
}

func NewPreprocessRunner(info *ArgParser, opts ...run.RunOption) run.Runner {
	r := &preprocessRunner{
		info: info,
	}
	r.Apply(opts...)
	return r
}

func (r *preprocessRunner) Run(ctx context.Context, tc *types.Toolchain) error {
	info := r.info
	if info.OutputArgIndex != -1 {
		r.Log.Debug("Replacing output path with '-'")
		old := info.Args[info.OutputArgIndex]
		info.ReplaceOutputPath("-")
		defer info.ReplaceOutputPath(old)
	}
	stderrBuf := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, tc.Executable, info.Args...)
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
	err := cmd.Start()
	if err != nil {
		return err
	}
	ch := make(chan struct{})
	defer close(ch)
	go func() {
		select {
		case <-r.Context.Done():
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
		r.Log.With(zap.Error(err)).Error("Compiler error")
		return run.NewCompilerError(stderrBuf.String())
	}
	return nil
}
