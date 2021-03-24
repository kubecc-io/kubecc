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
	"reflect"

	. "github.com/cobalt77/kubecc/pkg/kubecc/internal"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
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
	Use:   "get kind [args...]",
	Short: "Get information about a Kubecc cluster",
}

func init() {
	GetCmd.AddCommand(getAgents)
	GetCmd.AddCommand(getConsumerds)
	GetCmd.AddCommand(getSchedulers)
	GetCmd.AddCommand(getMonitors)
	GetCmd.AddCommand(getCaches)
	GetCmd.AddCommand(getMetricListeners)
	GetCmd.AddCommand(getComponents)
	GetCmd.AddCommand(getBuckets)
	GetCmd.AddCommand(getKeys)
	GetCmd.AddCommand(getMetrics)

	getMetrics.Flags().StringVarP(&outputKind, "output", "o", "text",
		"Output format. One of [text, json, jsonfmt]")
}

func getProviders() (*metrics.Providers, error) {
	c := client()
	m, err := c.GetMetric(CLIContext, &types.Key{
		Bucket: metrics.MetaBucket,
		Name:   "type.googleapis.com/metrics.Providers",
	})
	if err != nil {
		return nil, err
	}
	providers, err := m.GetValue().UnmarshalNew()
	return providers.(*metrics.Providers), err
}

func filterProviders(providers *metrics.Providers, c types.Component) []*metrics.ProviderInfo {
	info := []*metrics.ProviderInfo{}
	for _, p := range providers.GetItems() {
		if p.GetComponent() == c {
			info = append(info, p)
		}
	}
	return info
}

func writeOutput(msgs interface{}) {
	if typ := reflect.TypeOf(msgs); typ.Kind() != reflect.Slice {
		panic("msgs must be a slice")
	}
	val := reflect.ValueOf(msgs)
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i)
		msg := item.Interface().(proto.Message)
		switch outputKind {
		case "text":
			fmt.Println(prototext.Format(msg))
		case "json":
			data, err := protojson.Marshal(msg)
			if err != nil {
				CLILog.With("key").Error(err)
				continue
			}
			fmt.Println(string(data))
		case "jsonfmt":
			fmt.Println(protojson.Format(msg))
		}
	}
}

var getAgents = &cobra.Command{
	Use:     "agents",
	Aliases: []string{"agent"},
	Run: func(cmd *cobra.Command, args []string) {
		providers, err := getProviders()
		if err != nil {
			CLILog.Error(err)
			return
		}
		all := filterProviders(providers, types.Agent)
		if len(args) == 0 {
			writeOutput(all)
		}
	},
}

var getConsumerds = &cobra.Command{
	Use:     "consumerds",
	Aliases: []string{"consumerd"},
	Run: func(cmd *cobra.Command, args []string) {
		providers, err := getProviders()
		if err != nil {
			CLILog.Error(err)
			return
		}
		all := filterProviders(providers, types.Consumerd)
		if len(args) == 0 {
			writeOutput(all)
		}
	},
}

var getSchedulers = &cobra.Command{
	Use:     "scheduler",
	Aliases: []string{"schedulers"},
	Run: func(cmd *cobra.Command, args []string) {
		providers, err := getProviders()
		if err != nil {
			CLILog.Error(err)
			return
		}
		all := filterProviders(providers, types.Scheduler)
		if len(args) == 0 {
			writeOutput(all)
		}
	},
}

var getMonitors = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"monitors"},
	Run: func(cmd *cobra.Command, args []string) {
		providers, err := getProviders()
		if err != nil {
			CLILog.Error(err)
			return
		}
		all := filterProviders(providers, types.Monitor)
		if len(args) == 0 {
			writeOutput(all)
		}
	},
}

var getCaches = &cobra.Command{
	Use:     "cache",
	Aliases: []string{"caches"},
	Run: func(cmd *cobra.Command, args []string) {
		providers, err := getProviders()
		if err != nil {
			CLILog.Error(err)
			return
		}
		all := filterProviders(providers, types.Cache)
		if len(args) == 0 {
			writeOutput(all)
		}
	},
}

var getMetricListeners = &cobra.Command{
	Use:     "listeners",
	Aliases: []string{"listener"},
	Long:    "Print a list of all active metric listeners",
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var getComponents = &cobra.Command{
	Use:     "components",
	Aliases: []string{"component"},
	Run: func(cmd *cobra.Command, args []string) {
		providers, err := getProviders()
		if err != nil {
			CLILog.Error(err)
			return
		}
		infos := []*metrics.ProviderInfo{}
		for _, p := range providers.GetItems() {
			infos = append(infos, p)
		}
		if len(args) == 0 {
			writeOutput(infos)
		}
	},
}

var outputKind string
var getMetrics = &cobra.Command{
	Use:     "metrics",
	Aliases: []string{"metric"},
	Long:    "Print the contents of one or more metrics in the monitor's key-value store",
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

var getBuckets = &cobra.Command{
	Use:  "buckets",
	Long: "Print a list of all buckets in the monitor's key-value store",
	Args: cobra.NoArgs,
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

var getKeys = &cobra.Command{
	Use:  "keys",
	Long: "Print a list of all keys in a given bucket",
	Args: cobra.ExactArgs(1),
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
