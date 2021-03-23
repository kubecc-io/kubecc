package commands

import (
	"context"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/identity"
	. "github.com/cobalt77/kubecc/pkg/kubecc/internal"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/ui"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
)

// StatusCmd represents the status command.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show overall cluster status",
	Run: func(cmd *cobra.Command, args []string) {
		cc, err := servers.Dial(CLIContext, CLIConfig.MonitorAddress,
			servers.WithTLS(!CLIConfig.DisableTLS))
		if err != nil {
			CLILog.Fatal(err)
		}
		ctx := meta.NewContext(
			meta.WithProvider(identity.Component, meta.WithValue(types.CLI)),
			meta.WithProvider(identity.UUID),
			meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(
				types.CLI,
				logkc.WithLogLevel(zapcore.ErrorLevel),
			))),
		)
		client := types.NewMonitorClient(cc)
		listener := clients.NewListener(ctx, client)
		display := ui.NewStatusDisplay()

		listener.OnProviderAdded(func(pctx context.Context, uuid string) {
			info, err := client.Whois(ctx, &types.WhoisRequest{
				UUID: uuid,
			})
			if err != nil {
				return
			}
			if info.Component != types.Agent && info.Component != types.Consumerd {
				return
			}

			display.AddAgent(pctx, info)
			listener.OnValueChanged(uuid, func(qp *metrics.UsageLimits) {
				display.Update(uuid, qp)
			})
			listener.OnValueChanged(uuid, func(ts *metrics.TaskStatus) {
				display.Update(uuid, ts)
			})
			<-pctx.Done()
		})

		display.Run()
	},
}
