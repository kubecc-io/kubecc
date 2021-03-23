package components

import (
	"errors"
	"fmt"

	mapset "github.com/deckarep/golang-set"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	components = map[string]*cobra.Command{
		AgentCmd.Name():      AgentCmd,
		ConsumerdCmd.Name():  ConsumerdCmd,
		MonitorCmd.Name():    MonitorCmd,
		SchedulerCmd.Name():  SchedulerCmd,
		ControllerCmd.Name(): ControllerCmd,
		CacheCmd.Name():      CacheCmd,
		"all": {
			Run: func(cmd *cobra.Command, args []string) {
				go AgentCmd.Run(AgentCmd, args)
				go ConsumerdCmd.Run(ConsumerdCmd, args)
				go MonitorCmd.Run(MonitorCmd, args)
				go SchedulerCmd.Run(SchedulerCmd, args)
				go CacheCmd.Run(CacheCmd, args)
			},
		},
		"servers": {
			Run: func(cmd *cobra.Command, args []string) {
				go MonitorCmd.Run(MonitorCmd, args)
				go SchedulerCmd.Run(SchedulerCmd, args)
				go CacheCmd.Run(CacheCmd, args)
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

var RunCmd = &cobra.Command{
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
