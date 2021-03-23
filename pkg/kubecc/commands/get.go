/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package commands

import (
	"fmt"

	. "github.com/cobalt77/kubecc/pkg/kubecc/internal"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

func client() types.MonitorClient {
	cc, err := servers.Dial(CLIContext, CLIConfig.MonitorAddress,
		servers.WithTLS(!CLIConfig.DisableTLS))
	if err != nil {
		CLILog.Fatal(err)
	}
	return types.NewMonitorClient(cc)
}

var GetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get information from the monitor's key-value store",
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
				CLILog.With("key", arg).Error(err)
				return
			}
			keys = append(keys, k)
		}
		for _, key := range keys {
			metric, err := c.GetMetric(CLIContext, key)
			if err != nil {
				CLILog.With("key", key.Canonical()).Error(err)
				continue
			}
			switch outputKind {
			case "text":
				fmt.Println(prototext.Format(metric.GetValue()))
			case "json":
				data, err := protojson.Marshal(metric.GetValue())
				if err != nil {
					CLILog.With("key").Error(err)
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
		buckets, err := c.GetBuckets(CLIContext, &types.Empty{})
		if err != nil {
			CLILog.Error(err)
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
		keys, err := c.GetKeys(CLIContext, &types.Bucket{
			Name: args[0],
		})
		if err != nil {
			CLILog.Error(err)
			return
		}
		for _, key := range keys.GetKeys() {
			fmt.Println(key.Canonical())
		}
	},
}

func init() {
	GetCmd.AddCommand(getBucketsCmd)
	GetCmd.AddCommand(getMetricCmd)
	GetCmd.AddCommand(getKeysCmd)

	getMetricCmd.Flags().StringVarP(&outputKind, "output", "o", "text",
		"Output format. One of [text, json, jsonfmt]")
}
