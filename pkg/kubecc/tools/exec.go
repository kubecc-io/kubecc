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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/containerd/console"
	. "github.com/kubecc-io/kubecc/pkg/kubecc/internal"
	"github.com/kubecc-io/kubecc/pkg/stream"
	"github.com/spf13/cobra"
)

var ExecCmd = &cobra.Command{
	Use:                   "exec ...",
	Short:                 "Run commands with the kubecc environment configured",
	Aliases:               []string{"x"},
	Args:                  cobra.ArbitraryArgs,
	DisableFlagsInUseLine: true,
	DisableFlagParsing:    true,
	PersistentPreRun:      InitCLI,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			return
		}
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

			read, write, err := os.Pipe()
			if err != nil {
				CLILog.Fatal(err)
			}
			command.Env = os.Environ()
			command.Stdin = os.Stdin
			command.Stdout = write
			command.Stderr = write

			ctx, ca := context.WithCancel(context.Background())
			defer ca()
			streamDone := make(chan struct{})
			go func() {
				stream.RenderLogStream(ctx, read, streamDone, stream.WithHeader(func(c console.Console, w io.Writer) {
					sz, _ := c.Size()
					fmt.Fprintln(w, strings.Repeat("-", int(sz.Width)))
				}), stream.WithMaxHeight(10))
			}()
			err = command.Run()
			write.Close()
			read.Close()
			<-streamDone
			if err != nil {
				os.Exit(command.ProcessState.ExitCode())
			}
		}
	},
}
