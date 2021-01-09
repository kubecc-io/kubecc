package main

import (
	"github.com/spf13/cobra"
)

var local bool

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the remote agent",
	Run: func(cmd *cobra.Command, args []string) {
		if local {
			startLocalAgent()
		} else {
			startRemoteAgent()
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&local, "local", false,
		"Run the local agent instead")
}
