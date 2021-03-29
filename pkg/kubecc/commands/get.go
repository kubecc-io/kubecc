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

	"github.com/kubecc-io/kubecc/pkg/clients"
	. "github.com/kubecc-io/kubecc/pkg/kubecc/internal"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

func monitorClient() types.MonitorClient {
	cc, err := servers.Dial(CLIContext, CLIConfig.MonitorAddress,
		servers.WithTLS(!CLIConfig.DisableTLS))
	if err != nil {
		CLILog.Fatal(err)
	}
	return types.NewMonitorClient(cc)
}

func schedulerClient() types.SchedulerClient {
	cc, err := servers.Dial(CLIContext, CLIConfig.SchedulerAddress,
		servers.WithTLS(!CLIConfig.DisableTLS))
	if err != nil {
		CLILog.Fatal(err)
	}
	return types.NewSchedulerClient(cc)
}

var GetCmd = &cobra.Command{
	Use:              "get kind [args...]",
	Short:            "Get information about a Kubecc cluster",
	PersistentPreRun: InitCLI,
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
	GetCmd.AddCommand(getRoutes)
	GetCmd.AddCommand(getHealth)

	GetCmd.PersistentFlags().StringVarP(&outputKind, "output", "o", "text",
		"Output format. One of [text, json, jsonfmt]")
}

func getProviders() (*metrics.Providers, error) {
	c := monitorClient()
	m, err := c.GetMetric(CLIContext, &types.Key{
		Bucket: clients.MetaBucket,
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

func formatOutput(msgs interface{}) {
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
			formatOutput(all)
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
			formatOutput(all)
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
			formatOutput(all)
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
			formatOutput(all)
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
			formatOutput(all)
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
			formatOutput(infos)
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
		c := monitorClient()
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
			formatOutput([]proto.Message{metric})
		}
	},
}

var getBuckets = &cobra.Command{
	Use:  "buckets",
	Long: "Print a list of all buckets in the monitor's key-value store",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c := monitorClient()
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
		c := monitorClient()
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

var getRoutes = &cobra.Command{
	Use:  "routes",
	Long: "Print the scheduler's route list",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c := schedulerClient()
		keys, err := c.GetRoutes(CLIContext, &types.Empty{})
		if err != nil {
			CLILog.Error(err)
			return
		}
		formatOutput([]proto.Message{keys})
	},
}

var getHealth = &cobra.Command{
	Use:  "health",
	Long: "Print the health of all components",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c := monitorClient()
		buckets, err := c.GetBuckets(CLIContext, &types.Empty{})
		if err != nil {
			CLILog.Error(err)
			return
		}
		for _, bucket := range buckets.GetBuckets() {
			keys, err := c.GetKeys(CLIContext, bucket)
			if err != nil {
				CLILog.Error(err)
				continue
			}
			for _, key := range keys.GetKeys() {
				if key.Name == "type.googleapis.com/metrics.Health" {
					metric, err := c.GetMetric(CLIContext, key)
					if err != nil {
						CLILog.With("key", key.Canonical()).Error(err)
						continue
					}
					whois, err := c.Whois(CLIContext, &types.WhoisRequest{
						UUID: bucket.Name,
					})
					if err != nil {
						CLILog.With("uuid", bucket.Name).Error(err)
						continue
					}
					fmt.Printf("%s [%s]\n",
						whois.Component.Color().Add(whois.Component.Name()), whois.UUID)
					formatOutput([]proto.Message{metric.GetValue()})
				}
			}
		}
	},
}
