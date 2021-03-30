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
	"context"
	"fmt"
	"os"

	"github.com/kubecc-io/kubecc/pkg/toolchains"
)

type SleepToolchainFinder struct{}

func (f SleepToolchainFinder) FindToolchains(
	ctx context.Context,
	_ ...toolchains.FindOption,
) *toolchains.Store {
	executable, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("Could not find the current executable: %s", err.Error()))
	}
	store := toolchains.NewStore()
	_, _ = store.Add(executable, SleepQuerier{})
	return store
}
