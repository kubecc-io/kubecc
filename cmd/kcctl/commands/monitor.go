package commands

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/mmeta"
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

var listenCmd = &cobra.Command{
	Use:   "listen key",
	Short: "Display the real-time value of a key in the monitor's key-value store",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cc, err := servers.Dial(cliContext, address)
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
		tb := &ui.TextBox{}

		listener.OnValueChanged(mmeta.Bucket, func(contents *mmeta.StoreContents) {
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
		}).OrExpired(func() metrics.RetryOptions {
			tb.SetText("-- KEY EXPIRED -- \n\n" + tb.Paragraph.Text)
			return metrics.NoRetry
		})
		tb.Run()
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.AddCommand(listenCmd)

	monitorCmd.Flags().StringVarP(&address, "address", "a", "127.0.0.1:9960",
		"Address to use to connect to the monitor (ip:port)")
}
