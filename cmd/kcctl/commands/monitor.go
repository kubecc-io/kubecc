package commands

import (
	"fmt"

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/ui"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

// monitorCmd represents the monitor command.
var monitorCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"mon"},
	Short:   "Commands to interact with the monitor",
}

func onValueChanged(tb *ui.TextBox) func(*metrics.StoreContents) {
	return func(contents *metrics.StoreContents) {
		tb.SetText(prototext.Format(contents))
	}
}

func client() types.MonitorClient {
	cc, err := servers.Dial(cliContext, cliConfig.MonitorAddress,
		servers.WithTLS(!cliConfig.DisableTLS))
	if err != nil {
		cliLog.Fatal(err)
	}
	return types.NewMonitorClient(cc)
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get information from the monitor's key-value store",
	Run: func(cmd *cobra.Command, args []string) {
		tb := &ui.TextBox{}
		listener := clients.NewListener(cliContext, client())

		listener.OnValueChanged(metrics.MetaBucket, onValueChanged(tb)).
			OrExpired(func() metrics.RetryOptions {
				tb.SetText("-- KEY EXPIRED -- \n\n" + tb.Paragraph.Text)
				return metrics.NoRetry
			})
		tb.Run()
	},
}

var outputKind string
var getMetricCmd = &cobra.Command{
	Use:     "metric",
	Aliases: []string{"metrics"},
	Short:   "Print the contents of one or more metrics in the monitor's key-value store",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c := client()
		keys := []*types.Key{}

		for _, arg := range args {
			k, err := types.ParseKey(arg)
			if err != nil {
				cliLog.With("key", arg).Error(err)
				return
			}
			keys = append(keys, k)
		}
		for _, key := range keys {
			metric, err := c.GetMetric(cliContext, key)
			if err != nil {
				cliLog.With("key", key.Canonical()).Error(err)
				continue
			}
			switch outputKind {
			case "text":
				fmt.Println(prototext.Format(metric.GetValue()))
			case "json":
				data, err := protojson.Marshal(metric.GetValue())
				if err != nil {
					cliLog.With("key").Error(err)
					continue
				}
				fmt.Println(string(data))
			case "jsonfmt":
				fmt.Println(protojson.Format(metric.GetValue()))
			}

		}

	},
}

var getBucketsCmd = &cobra.Command{
	Use:   "buckets",
	Short: "Print a list of all buckets in the monitor's key-value store",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c := client()
		buckets, err := c.GetBuckets(cliContext, &types.Empty{})
		if err != nil {
			cliLog.Error(err)
			return
		}
		for _, bucket := range buckets.GetBuckets() {
			fmt.Println(bucket.GetName())
		}
	},
}

var getKeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Print a list of all keys in a given bucket",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c := client()
		keys, err := c.GetKeys(cliContext, &types.Bucket{
			Name: args[0],
		})
		if err != nil {
			cliLog.Error(err)
			return
		}
		for _, key := range keys.GetKeys() {
			fmt.Println(key.Canonical())
		}
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.AddCommand(getCmd)
	getCmd.AddCommand(getBucketsCmd)
	getCmd.AddCommand(getMetricCmd)
	getCmd.AddCommand(getKeysCmd)

	getMetricCmd.Flags().StringVarP(&outputKind, "output", "o", "text",
		"Output format. One of [text, json, jsonfmt]")
}
