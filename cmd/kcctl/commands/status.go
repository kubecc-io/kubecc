package commands

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/meta"
	"github.com/cobalt77/kubecc/pkg/metrics/status"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
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
		id := types.NewIdentity(types.CLI)
		ctx := types.OutgoingContextWithIdentity(cliContext, id)
		client := types.NewExternalMonitorClient(cc)
		listener := metrics.NewListener(ctx, client)
		listener.OnProviderAdded(func(pctx context.Context, uuid string) {
			lg := cliLog.With("uuid", uuid)
			lg.Info("Provider added")
			listener.OnValueChanged(uuid, func(*meta.Alive) {
				lg.Info("Alive")
			}).OrExpired(func() metrics.RetryOptions {
				lg.Info("Expired")
				return metrics.NoRetry
			})
			listener.OnValueChanged(uuid, func(qp *status.QueueParams) {
				lg.With(zap.Any("params", qp)).Info("Queue params")
			})
			listener.OnValueChanged(uuid, func(qs *status.QueueStatus) {
				lg.With(zap.Any("status", types.QueueStatus(qs.QueueStatus).String())).
					Info("Queue status")
			})
			listener.OnValueChanged(uuid, func(ts *status.TaskStatus) {
				lg.With(zap.Any("status", ts)).Info("Task status")
			})
			<-pctx.Done()
			lg.Info("Provider removed")
		})
		select {
		case <-ctrl.SetupSignalHandler().Done():
		case <-ctx.Done():
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
