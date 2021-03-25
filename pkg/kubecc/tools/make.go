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
	"strings"

	. "github.com/cobalt77/kubecc/pkg/kubecc/internal"
	"github.com/spf13/cobra"
)

func pathToMake() string {
	locations := []string{"/usr/bin/make", "/bin/make"}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	fmt.Fprintf(os.Stderr, "make not found (checked %s)", strings.Join(locations, ", "))
	os.Exit(1)
	return ""
}

var MakeCmd = &cobra.Command{
	Use:   "make",
	Short: "A wrapper around make",
	Long: `A wrapper around make that will allow all sub-processes to be grouped for tracing purposes.
This tool will automatically be run if the kubecc binary is invoked with the name 'make'.`,
	PersistentPreRun: InitCLI,
	Run: func(_ *cobra.Command, args []string) {
		cmd := exec.Command(pathToMake(), args...)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("KUBECC_MAKE_PID=%d", os.Getpid()))
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running make: %s\n", err)
			os.Exit(1)
		}
		if err := cmd.Wait(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	},
}
