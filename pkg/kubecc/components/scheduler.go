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

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/apps/scheduler"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	_ "google.golang.org/grpc/encoding/gzip"
)

func runScheduler(cmd *cobra.Command, args []string) {
	conf := config.ConfigMapProvider.Load().Scheduler

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Scheduler)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.Scheduler,
				logkc.WithLogLevel(conf.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
	)
	lg := meta.Log(ctx)

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		panic(err.Error())
	}
	lg.With("addr", listener.Addr().String()).
		Info("Server listening")

	monitorCC, err := servers.Dial(ctx, conf.MonitorAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing monitor")
	}
	lg.With("address", monitorCC.Target()).Info("Dialing monitor")

	cacheCC, err := servers.Dial(ctx, conf.CacheAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing cache server")
	}
	lg.With("address", cacheCC.Target()).Info("Dialing cache server")

	monitorClient := types.NewMonitorClient(monitorCC)
	cacheClient := types.NewCacheClient(cacheCC)

	sc := scheduler.NewSchedulerServer(ctx,
		scheduler.WithMonitorClient(monitorClient),
		scheduler.WithCacheClient(cacheClient),
	)
	types.RegisterSchedulerServer(srv, sc)
	go sc.StartMetricsProvider()

	err = srv.Serve(listener)
	if err != nil {
		lg.With(zap.Error(err)).Error("GRPC error")
	}
}

var SchedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Run the scheduler server",
	Run:   runScheduler,
}
