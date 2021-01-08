package main

import (
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the remote agent",
	Run: func(cmd *cobra.Command, args []string) {
		StartRemoteAgent()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
