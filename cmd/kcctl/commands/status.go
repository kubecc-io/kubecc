package commands

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/common"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/ui"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		cc, err := servers.Dial(cliContext, "127.0.0.1:9960")
		if err != nil {
			cliLog.Fatal(err)
		}
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger),
		)
		client := types.NewExternalMonitorClient(cc)
		listener := metrics.NewListener(ctx, client)
		display := ui.NewStatusDisplay()

		listener.OnProviderAdded(func(pctx context.Context, uuid string) {
			display.AddAgent(pctx, uuid)
			listener.OnValueChanged(uuid, func(qp *common.QueueParams) {
				display.Update(uuid, qp)
			})
			listener.OnValueChanged(uuid, func(qs *common.QueueStatus) {
				display.Update(uuid, qs)
			})
			listener.OnValueChanged(uuid, func(ts *common.TaskStatus) {
				display.Update(uuid, ts)
			})
			<-pctx.Done()
		})

		display.Run()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
