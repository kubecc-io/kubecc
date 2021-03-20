package run

import (
	"io"
	"os/exec"

	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
)

type ExecCommandTask struct {
	TaskOptions
	util.NullableError
	Toolchain *types.Toolchain
	Args      []string
	Env       []string
	WorkDir   string
	Stdout    io.Writer
	Stderr    io.Writer
	Stdin     io.Reader
}

func (t *ExecCommandTask) Run() {
	cmd := exec.CommandContext(t.Context, t.Toolchain.Executable, t.Args...)
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
