package commands

import (
	"encoding/json"
	"strings"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/ui"
	"github.com/spf13/cobra"
)

// monitorCmd represents the monitor command.
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Commands to interact with the monitor",
}

func onValueChanged(tb *ui.TextBox) func(*metrics.StoreContents) {
	return func(contents *metrics.StoreContents) {
		printable := map[string]interface{}{}
		for _, bucket := range contents.Buckets {
			jsonContents := map[string]string{}
			for k, v := range bucket.Data {
				jsonContents[k] = v.String()
			}
			printable[bucket.Name] = jsonContents
		}
		data, err := json.MarshalIndent(printable, "", " ")
		str := strings.ReplaceAll(string(data), `\"`, `"`)
		if err != nil {
			tb.SetText(err.Error())
		} else {
			tb.SetText(str)
		}
	}
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Display the real-time contents of the monitor's key-value store",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cc, err := servers.Dial(cliContext, cliConfig.MonitorAddress,
			servers.WithTLS(!cliConfig.DisableTLS))
		if err != nil {
			cliLog.Fatal(err)
		}
		client := types.NewMonitorClient(cc)
		listener := clients.NewListener(cliContext, client)
		tb := &ui.TextBox{}

		listener.OnValueChanged(metrics.MetaBucket, onValueChanged(tb)).
			OrExpired(func() metrics.RetryOptions {
				tb.SetText("-- KEY EXPIRED -- \n\n" + tb.Paragraph.Text)
				return metrics.NoRetry
			})
		tb.Run()
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.AddCommand(listenCmd)
}
