package kubecc

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/kubecc/commands"
	"github.com/cobalt77/kubecc/pkg/kubecc/components"
	"github.com/cobalt77/kubecc/pkg/kubecc/tools"
)

func CreateRootCmd() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:  "kubecc",
		Long: logkc.BigAsciiTextColored,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

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
	templates.ActsAsRootCommand(rootCmd, nil, groups...)
	return rootCmd
}

func Execute() {
	if err := CreateRootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
