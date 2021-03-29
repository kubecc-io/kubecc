/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package run

import (
	"os/exec"

	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
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
