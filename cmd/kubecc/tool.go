package main

import (
	"github.com/cobalt77/kubecc/cmd/kubecc/tools"
	"github.com/spf13/cobra"
)

var toolCmd = &cobra.Command{
	Use:   "tool command",
	Short: "Run a built-in utility or debugging tool.",
}

func init() {
	toolCmd.AddCommand(tools.SleepCmd)
	toolCmd.AddCommand(tools.MakeSleepCmd)

	rootCmd.AddCommand(toolCmd)
}
