package cachesrv

import (
	"context"
	"time"

	csrvmetrics "github.com/cobalt77/kubecc/pkg/apps/cachesrv/metrics"
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
	client types.MonitorClient,
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
		srv.metricsProvider = metrics.NewMonitorProvider(
			ctx, options.monitorClient, metrics.Buffered|metrics.Block)
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
	s.postStorageProvider()

	slowTimer := util.NewJitteredTimer(20*time.Second, 0.5)
	go func() {
		for {
			<-slowTimer
			s.postStorageInfo()
			s.postCacheHits()
			s.postStorageProvider()
		}
	}()
}
