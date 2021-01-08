package cmd

import (
	"github.com/cobalt77/kube-distcc/agent"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the remote agent",
	Run: func(cmd *cobra.Command, args []string) {
		agent.StartRemoteAgent()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
