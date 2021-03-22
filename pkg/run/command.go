package run

import (
	"os/exec"

	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
)

type ExecCommandTask struct {
	TaskOptions
	util.NullableError
	tc *types.Toolchain
}

func NewExecCommandTask(tc *types.Toolchain, opts ...TaskOption) Task {
	r := &ExecCommandTask{
		tc: tc,
	}
	r.Apply(opts...)
	return r
}

func (t *ExecCommandTask) Run() {
	cmd := exec.CommandContext(t.Context, t.tc.Executable, t.Args...)
	cmd.Env = t.Env
	cmd.Dir = t.WorkDir
	cmd.Stdout = t.Stdout
	cmd.Stderr = t.Stderr
	cmd.Stdin = t.Stdin
	err := cmd.Start()
	if err != nil {
		t.SetErr(err)
		return
	}
	err = cmd.Wait()
	if err != nil {
		t.SetErr(err)
		return
	}
	t.SetErr(nil)
}
