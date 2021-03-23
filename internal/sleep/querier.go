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

package sleep

import (
	"runtime"
	"time"

	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/types"
)

type SleepQuerier struct{}

func (q SleepQuerier) IsPicDefault(compiler string) (bool, error) {
	return true, nil
}

func (q SleepQuerier) TargetArch(compiler string) (string, error) {
	return runtime.GOARCH, nil
}

func (q SleepQuerier) Version(compiler string) (string, error) {
	return "1.0", nil
}

func (q SleepQuerier) Kind(compiler string) (types.ToolchainKind, error) {
	return types.Sleep, nil
}

func (q SleepQuerier) Lang(compiler string) (types.ToolchainLang, error) {
	return types.CXX, nil
}

func (q SleepQuerier) ModTime(compiler string) (time.Time, error) {
	return toolchains.ExecQuerier{}.ModTime(compiler)
}
