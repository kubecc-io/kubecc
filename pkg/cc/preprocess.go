package cc

import (
	"bytes"
	"io"
	"os/exec"
	"syscall"

	"github.com/cobalt77/kubecc/pkg/run"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"go.uber.org/zap"
)

type preprocessTask struct {
	util.NullableError
	run.TaskOptions

	tc *types.Toolchain
	ap *ArgParser
}

func NewPreprocessTask(tc *types.Toolchain, ap *ArgParser, opts ...run.TaskOption) run.Task {
	r := &preprocessTask{
		tc: tc,
		ap: ap,
	}
	r.Apply(opts...)
	return r
}

func (t *preprocessTask) Run() {
	info := t.ap
	if info.OutputArgIndex != -1 {
		t.Log.Debug("Replacing output path with '-'")
		old := info.Args[info.OutputArgIndex]
		info.ReplaceOutputPath("-")
		defer info.ReplaceOutputPath(old)
	}
	stderrBuf := new(bytes.Buffer)
	cmd := exec.CommandContext(t.Context, t.tc.Executable, info.Args...)
	cmd.Env = t.Env
	cmd.Dir = t.WorkDir
	cmd.Stdout = t.OutputWriter
	cmd.Stderr = io.MultiWriter(t.Stderr, stderrBuf)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         t.UID,
			Gid:         t.GID,
			NoSetGroups: true,
		},
	}
	err := cmd.Start()
	if err != nil {
		t.SetErr(err)
		return
	}
	ch := make(chan struct{})
	defer close(ch)
	go func() {
		select {
		case <-t.Context.Done():
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
		t.Log.With(zap.Error(err)).Error("Compiler error")
		t.SetErr(run.NewCompilerError(stderrBuf.String()))
		return
	}
	t.SetErr(nil)
	return
}
