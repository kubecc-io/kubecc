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

package kubecc

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/kubecc/commands"
	"github.com/kubecc-io/kubecc/pkg/kubecc/components"
	. "github.com/kubecc-io/kubecc/pkg/kubecc/internal"
	"github.com/kubecc-io/kubecc/pkg/kubecc/tools"
)

func CreateRootCmd() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:  "kubecc",
		Long: logkc.BigAsciiTextColored,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	rootCmd.PersistentFlags().StringVar(&ConfigPath, "config", "",
		"Path to config file. If not set, uses default locations (~/.kubecc/config.yaml, /etc/kubecc/config.yaml)")

	groups := templates.CommandGroups{
		{
			Message: "Server Components:",
			Commands: []*cobra.Command{
				components.AgentCmd,
				components.CacheCmd,
				components.ConsumerdCmd,
				components.MonitorCmd,
				components.SchedulerCmd,
				components.ControllerCmd,
				components.RunCmd,
			},
		},
		{
			Message: "Tools:",
			Commands: []*cobra.Command{
				tools.ConsumerCmd,
				tools.MakeCmd,
				tools.SleepCmd,
				tools.MakeSleepCmd,
			},
		},
		{
			Message: "Status and Management:",
			Commands: []*cobra.Command{
				commands.StatusCmd,
				commands.GetCmd,
			},
		},
	}
	groups.Add(rootCmd)
	fe := templates.ActsAsRootCommand(rootCmd, nil, groups...)
	fe.ExposeFlags(rootCmd, "config")
	return rootCmd
}

func Execute() {
	if err := CreateRootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
