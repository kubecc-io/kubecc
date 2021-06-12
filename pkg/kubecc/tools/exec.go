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

package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	. "github.com/kubecc-io/kubecc/pkg/kubecc/internal"
	"github.com/spf13/cobra"
)

var ExecCmd = &cobra.Command{
	Use:     "exec ...",
	Short:   "Run commands with the kubecc environment configured",
	Aliases: []string{"x"},
	Args:    cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		command := exec.Command(args[0], args[1:]...)
		wd, err := os.Getwd()
		if err != nil {
			CLILog.Fatal(err)
		}
		if home, ok := os.LookupEnv("KUBECC_HOME"); !ok {
			CLILog.Fatal("KUBECC_HOME not set. Try running 'kubecc setup' first.")
		} else {
			command.Dir = wd
			if err := os.Setenv("PATH", fmt.Sprintf("%s:%s", path.Join(home, "bin"), os.Getenv("PATH"))); err != nil {
				CLILog.Fatal(err)
			}
			command.Env = os.Environ()
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin
			if err := command.Run(); err != nil {
				os.Exit(command.ProcessState.ExitCode())
			}
		}
	},
}
