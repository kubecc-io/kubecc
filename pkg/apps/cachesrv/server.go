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

package cachesrv

import (
	"context"
	"time"

	"github.com/kubecc-io/kubecc/pkg/clients"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/storage"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/kubecc-io/kubecc/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CacheServer struct {
	metrics.StatusController
	types.UnimplementedCacheServer
	srvContext      context.Context
	lg              *zap.SugaredLogger
	cfg             config.CacheSpec
	metricsProvider clients.MetricsProvider
	storageProvider storage.StorageProvider
}

type CacheServerOptions struct {
	storageProvider storage.StorageProvider
	monitorClient   types.MonitorClient
}

type CacheServerOption func(*CacheServerOptions)

func (o *CacheServerOptions) Apply(opts ...CacheServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithStorageProvider(sp storage.StorageProvider) CacheServerOption {
	return func(o *CacheServerOptions) {
		o.storageProvider = sp
	}
}

func WithMonitorClient(
	client types.MonitorClient,
) CacheServerOption {
	return func(o *CacheServerOptions) {
		o.monitorClient = client
	}
}
func NewCacheServer(
	ctx context.Context,
	cfg config.CacheSpec,
	opts ...CacheServerOption,
) *CacheServer {
	options := CacheServerOptions{}
	options.Apply(opts...)
	srv := &CacheServer{
		srvContext: ctx,
		cfg:        cfg,
		lg:         meta.Log(ctx),
	}
	srv.BeginInitialize()
	defer srv.EndInitialize()

	if options.storageProvider == nil {
		srv.ApplyCondition(ctx, metrics.StatusConditions_InvalidConfiguration,
			"No storage provider set")
		srv.lg.Error("No storage provider set.")
	} else {
		srv.storageProvider = options.storageProvider
		if options.monitorClient != nil {
			srv.metricsProvider = clients.NewMetricsProvider(
				ctx, options.monitorClient, clients.Buffered|clients.Block,
				clients.StatusCtrl(&srv.StatusController))
		} else {
			srv.metricsProvider = clients.NewNoopMetricsProvider()
		}
		if err := srv.storageProvider.Configure(); err != nil {
			srv.ApplyCondition(ctx, metrics.StatusConditions_InvalidConfiguration,
				err.Error())
			srv.lg.Error(err)
		}
	}
	return srv
}

func (s *CacheServer) Push(
	ctx context.Context,
	req *types.PushRequest,
) (*types.Empty, error) {
	return &types.Empty{}, s.storageProvider.Put(ctx, req.Key, req.Object)
}

func (s *CacheServer) Pull(
	ctx context.Context,
	req *types.PullRequest,
) (*types.CacheObject, error) {
	s.lg.Debug("Handling pull request")
	return s.storageProvider.Get(ctx, req.Key)
}

func (s *CacheServer) Query(
	ctx context.Context,
	req *types.QueryRequest,
) (*types.QueryResponse, error) {
	s.lg.Debug("Handling query request")
	resp, err := s.storageProvider.Query(ctx, req.Keys)
	return &types.QueryResponse{
		Results: resp,
	}, err
}

func (s *CacheServer) Sync(*types.SyncRequest, types.Cache_SyncServer) error {
	return status.Errorf(codes.Unimplemented, "method Sync not implemented")
}

func (s *CacheServer) postStorageInfo() {
	s.metricsProvider.Post(s.storageProvider.UsageInfo())
}

func (s *CacheServer) postCacheHits() {
	s.metricsProvider.Post(s.storageProvider.CacheHits())
}

func (s *CacheServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")

	slowTimer := util.NewJitteredTimer(3*time.Second, 0.5) // 3-4.5s
	go func() {
		for {
			<-slowTimer
			s.postStorageInfo()
			s.postCacheHits()
		}
	}()
}
