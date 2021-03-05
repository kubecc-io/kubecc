package cachesrv

import (
	"context"
	"time"

	csrvmetrics "github.com/cobalt77/kubecc/pkg/apps/cachesrv/metrics"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/metrics/common"
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
	monitorClient   types.InternalMonitorClient
}

type CacheServerOptions struct {
	storageProvider storage.StorageProvider
	monitorClient   types.InternalMonitorClient
}

type cacheServerOption func(*CacheServerOptions)

func (o *CacheServerOptions) Apply(opts ...cacheServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithStorageProvider(sp storage.StorageProvider) cacheServerOption {
	return func(o *CacheServerOptions) {
		o.storageProvider = sp
	}
}

func WithMonitorClient(
	client types.InternalMonitorClient,
) cacheServerOption {
	return func(o *CacheServerOptions) {
		o.monitorClient = client
	}
}
func NewCacheServer(
	ctx context.Context,
	cfg config.CacheSpec,
	opts ...cacheServerOption,
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
		srv.metricsProvider = metrics.NewMonitorProvider(ctx, options.monitorClient)
	} else {
		srv.metricsProvider = metrics.NewNoopProvider()
	}
	return srv
}

func (s *CacheServer) Run() {
	if err := s.storageProvider.Configure(); err != nil {
		s.lg.Error(err)
	}
}

func (s *CacheServer) Push(
	ctx context.Context,
	req *types.PushRequest,
) (*types.Empty, error) {
	s.lg.Debug("Handling push request")
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

func (s *CacheServer) postAlive() {
	s.metricsProvider.Post(&common.Alive{})
}

func (s *CacheServer) postStorageProvider() {
	s.metricsProvider.Post(&csrvmetrics.StorageProvider{
		Kind: s.storageProvider.Location().String(),
	})
}

func (s *CacheServer) postStorageInfo() {
	s.metricsProvider.Post(s.storageProvider.UsageInfo())
}

func (s *CacheServer) postCacheHits() {
	s.metricsProvider.Post(s.storageProvider.CacheHits())
}

func (s *CacheServer) StartMetricsProvider() {
	s.lg.Info("Starting metrics provider")
	s.postAlive()
	s.postStorageProvider()

	slowTimer := util.NewJitteredTimer(1*time.Second, 1.0)
	go func() {
		for {
			<-slowTimer
			s.postStorageInfo()
			s.postCacheHits()
		}
	}()
}
