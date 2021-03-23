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

package components

import (
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/sleep"
	sleepctrl "github.com/cobalt77/kubecc/internal/sleep/controller"
	"github.com/cobalt77/kubecc/pkg/apps/consumerd"
	"github.com/cobalt77/kubecc/pkg/cc"
	ccctrl "github.com/cobalt77/kubecc/pkg/cc/controller"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/host"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/servers"
	"github.com/cobalt77/kubecc/pkg/toolchains"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func runConsumerd(cmd *cobra.Command, args []string) {
	conf := (&config.ConfigMapProvider{}).Load().Consumerd

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Consumerd)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.Consumerd,
				logkc.WithLogLevel(conf.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)

	schedulerCC, err := servers.Dial(ctx, conf.SchedulerAddress,
		servers.WithTLS(!conf.DisableTLS),
	)
	lg.With("address", schedulerCC.Target()).Info("Dialing scheduler")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing scheduler")
	}

	monitorCC, err := servers.Dial(ctx, conf.MonitorAddress,
		servers.WithTLS(!conf.DisableTLS))
	lg.With("address", monitorCC.Target()).Info("Dialing monitor")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing monitor")
	}

	schedulerClient := types.NewSchedulerClient(schedulerCC)
	monitorClient := types.NewMonitorClient(monitorCC)
	srv := servers.NewServer(ctx)

	d := consumerd.NewConsumerdServer(ctx,
		consumerd.WithUsageLimits(&metrics.UsageLimits{
			ConcurrentProcessLimit:  int32(conf.UsageLimits.ConcurrentProcessLimit),
			QueuePressureMultiplier: conf.UsageLimits.QueuePressureMultiplier,
			QueueRejectMultiplier:   conf.UsageLimits.QueueRejectMultiplier,
		}),
		consumerd.WithToolchainFinders(
			toolchains.FinderWithOptions{
				Finder: cc.CCFinder{},
			},
			toolchains.FinderWithOptions{
				Finder: sleep.SleepToolchainFinder{},
			},
		),
		consumerd.WithToolchainRunners(ccctrl.AddToStore, sleepctrl.AddToStore),
		consumerd.WithSchedulerClient(schedulerClient),
		consumerd.WithMonitorClient(monitorClient),
	)
	types.RegisterConsumerdServer(srv, d)
	go d.StartMetricsProvider()

	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		lg.With(
			zap.Error(err),
			zap.String("address", conf.ListenAddress),
		).Fatal("Error listening on socket")
	}
	lg.With("addr", listener.Addr()).Info("Listening")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Could not start consumerd")
	}

	err = srv.Serve(listener)
	if err != nil {
		lg.Error(err.Error())
	}
}

var ConsumerdCmd = &cobra.Command{
	Use:   "consumerd",
	Short: "Run the consumerd server",
	Run:   runConsumerd,
}
