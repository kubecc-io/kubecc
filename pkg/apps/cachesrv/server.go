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

	"github.com/cobalt77/kubecc/pkg/clients"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/storage"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/cobalt77/kubecc/pkg/util"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CacheServer struct {
	types.UnimplementedCacheServer
	srvContext      context.Context
	lg              *zap.SugaredLogger
	cfg             config.CacheSpec
	metricsProvider metrics.Provider
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
	if options.storageProvider == nil {
		srv.lg.Fatal("No storage provider set.")
	}
	srv.storageProvider = options.storageProvider
	if options.monitorClient != nil {
		srv.metricsProvider = clients.NewMonitorProvider(
			ctx, options.monitorClient, clients.Buffered|clients.Block)
	} else {
		srv.metricsProvider = metrics.NewNoopProvider()
	}
	if err := srv.storageProvider.Configure(); err != nil {
		srv.lg.Fatal(err)
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

	slowTimer := util.NewJitteredTimer(20*time.Second, 0.5)
	go func() {
		for {
			<-slowTimer
			s.postStorageInfo()
			s.postCacheHits()
		}
	}()
}
