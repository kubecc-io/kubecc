package commands

import (
	"bytes"
	"encoding/json"
	"strings"

	monitormetrics "github.com/cobalt77/kubecc/pkg/apps/monitor/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/ui"
	"github.com/spf13/cobra"
	"github.com/tinylib/msgp/msgp"
)

var address string

// monitorCmd represents the monitor command.
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Commands to interact with the monitor",
}

func onValueChanged(tb *ui.TextBox) func(*monitormetrics.StoreContents) {
	return func(contents *monitormetrics.StoreContents) {
		printable := map[string]interface{}{}
		for _, bucket := range contents.Buckets {
			jsonContents := map[string]string{}
			for k, v := range bucket.Data {
				buf := new(bytes.Buffer)
				_, err := msgp.UnmarshalAsJSON(buf, v)
				if err != nil {
					jsonContents[k] = "<error>"
				} else {
					jsonContents[k] = buf.String()
				}
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
	Use:   "listen key",
	Short: "Display the real-time value of a key in the monitor's key-value store",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cc, err := servers.Dial(cliContext, address, servers.WithTLS(true))
		if err != nil {
			cliLog.Fatal(err)
		}
		client := types.NewExternalMonitorClient(cc)
		listener := metrics.NewListener(cliContext, client)
		tb := &ui.TextBox{}

		listener.OnValueChanged(monitormetrics.MetaBucket, onValueChanged(tb)).
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

	monitorCmd.PersistentFlags().StringVarP(&address, "address", "a", "",
		"Address to use to connect to the monitor (ip:port)")
}
