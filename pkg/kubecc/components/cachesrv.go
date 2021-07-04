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
	"github.com/kubecc-io/kubecc/pkg/apps/cachesrv"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/storage"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func runCache(cmd *cobra.Command, args []string) {
	conf := config.ConfigMapProvider.Load().Cache

	ctx := meta.NewContext(
		meta.WithProvider(identity.Component, meta.WithValue(types.Cache)),
		meta.WithProvider(identity.UUID),
		meta.WithProvider(logkc.Logger, meta.WithValue(
			logkc.New(
				types.Cache,
				logkc.WithLogLevel(conf.LogLevel.Level()),
			),
		)),
		meta.WithProvider(tracing.Tracer),
		meta.WithProvider(host.SystemInfo),
	)
	lg := meta.Log(ctx)

	srv := servers.NewServer(ctx)
	listener, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		lg.With(zap.Error(err)).Fatalw("Error listening on socket")
	}
	lg.With("addr", listener.Addr().String()).Info("Server listening")

	monitorCC, err := servers.Dial(ctx, conf.MonitorAddress)
	lg.With("address", monitorCC.Target()).Info("Dialing monitor")
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Error dialing monitor")
	}

	providers := []storage.StorageProvider{}
	// order is important here, this is the priority order for the chain
	// storage provider
	if conf.VolatileStorage != nil {
		providers = append(providers,
			storage.NewVolatileStorageProvider(ctx, *conf.VolatileStorage))
	}
	if conf.LocalStorage != nil {
		providers = append(providers,
			storage.NewLocalStorageProvider(ctx, *conf.LocalStorage))
	}
	if conf.RemoteStorage != nil {
		providers = append(providers,
			storage.NewS3StorageProvider(ctx, *conf.RemoteStorage))
	}
	cacheSrv := cachesrv.NewCacheServer(ctx, conf,
		cachesrv.WithStorageProvider(
			storage.NewChainStorageProvider(ctx, providers...),
		),
		cachesrv.WithMonitorClient(types.NewMonitorClient(monitorCC)),
	)
	types.RegisterCacheServer(srv, cacheSrv)

	go cacheSrv.StartMetricsProvider()

	err = srv.Serve(listener)
	if err != nil {
		lg.With(zap.Error(err)).Error("GRPC error")
	}
}

var CacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Run the cache server",
	Run:   runCache,
}
