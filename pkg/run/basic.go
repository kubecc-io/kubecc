package run

import (
	"context"
	"io"
	"os/exec"

	"github.com/cobalt77/kubecc/pkg/types"
)

type BasicExecutableRunner struct {
	Args    []string
	Env     []string
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
}

func (r *BasicExecutableRunner) Run(ctx context.Context, tc *types.Toolchain) error {
	cmd := exec.CommandContext(ctx, tc.Executable, r.Args...)
	cmd.Env = r.Env
	cmd.Dir = r.WorkDir
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Stdin = r.Stdin
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}
