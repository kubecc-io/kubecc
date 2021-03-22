package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	agentcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/agent"
	cachecmd "github.com/cobalt77/kubecc/cmd/kubecc/components/cachesrv"
	cdcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/consumerd"
	ctrlcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/controller"
	moncmd "github.com/cobalt77/kubecc/cmd/kubecc/components/monitor"
	schedcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/scheduler"
	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/cluster"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:  "kubecc",
	Long: logkc.BigAsciiTextColored,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(agentcmd.Command)
	rootCmd.AddCommand(cdcmd.Command)
	rootCmd.AddCommand(moncmd.Command)
	rootCmd.AddCommand(schedcmd.Command)
	rootCmd.AddCommand(ctrlcmd.Command)
	rootCmd.AddCommand(cachecmd.Command)
	if !cluster.InCluster() {
		ctrlcmd.Command.Hidden = true
	}
}
