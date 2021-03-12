package main

import (
	"errors"
	"fmt"

	agentcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/agent"
	cachecmd "github.com/cobalt77/kubecc/cmd/kubecc/components/cachesrv"
	cdcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/consumerd"
	ctrlcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/controller"
	moncmd "github.com/cobalt77/kubecc/cmd/kubecc/components/monitor"
	schedcmd "github.com/cobalt77/kubecc/cmd/kubecc/components/scheduler"
	mapset "github.com/deckarep/golang-set"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	components = map[string]*cobra.Command{
		agentcmd.Command.Name(): agentcmd.Command,
		cdcmd.Command.Name():    cdcmd.Command,
		moncmd.Command.Name():   moncmd.Command,
		schedcmd.Command.Name(): schedcmd.Command,
		ctrlcmd.Command.Name():  ctrlcmd.Command,
		cachecmd.Command.Name(): cachecmd.Command,
		"all": {
			Run: func(cmd *cobra.Command, args []string) {
				go agentcmd.Command.Run(agentcmd.Command, args)
				go cdcmd.Command.Run(agentcmd.Command, args)
				go moncmd.Command.Run(agentcmd.Command, args)
				go schedcmd.Command.Run(agentcmd.Command, args)
				go cachecmd.Command.Run(agentcmd.Command, args)
			},
		},
		"servers": {
			Run: func(cmd *cobra.Command, args []string) {
				go moncmd.Command.Run(agentcmd.Command, args)
				go schedcmd.Command.Run(agentcmd.Command, args)
				go cachecmd.Command.Run(agentcmd.Command, args)
			},
		},
	}
	componentNames []string

	UnknownComponent = errors.New("Unknown component")
)

func init() {
	for k := range components {
		componentNames = append(componentNames, k)
	}
}

var runCmd = &cobra.Command{
	Use:       "run component...",
	Short:     "Run one or more kubecc components.",
	ValidArgs: append(componentNames, "all"),
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cobra.MinimumNArgs(1)(cmd, args)
		}
		for _, arg := range args {
			if _, ok := components[arg]; !ok {
				return fmt.Errorf("%w: %s", UnknownComponent, arg)
			}
		}
		return nil
	},
	Example: `run agent             # runs only the agent
run monitor scheduler # runs both the monitor and scheduler
run all               # runs monitor, scheduler, agent, and consumerd`,
	SuggestFor: []string{"start"},
	RunE: func(cmd *cobra.Command, args []string) error {
		set := mapset.NewSetFromSlice(func() []interface{} {
			argValues := make([]interface{}, len(args))
			for i, arg := range args {
				argValues[i] = arg
			}
			return argValues
		}())
		if set.Cardinality() < len(args) {
			fmt.Println("Warning: Duplicate components ignored")
		}
		for item := range set.Iter() {
			cmd := components[item.(string)]
			go cmd.Run(cmd, []string{})
		}
		<-ctrl.SetupSignalHandler().Done()
		return nil
	},
}
