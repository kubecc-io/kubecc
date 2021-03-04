package cachesrv

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/storage"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CacheServer struct {
	types.UnimplementedCacheServer
	srvContext      context.Context
	lg              *zap.SugaredLogger
	cfg             config.CacheSpec
	storageProvider storage.StorageProvider
}

type CacheServerOptions struct {
	storageProvider storage.StorageProvider
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
	return &types.Empty{}, s.storageProvider.Put(ctx, req.Key, req.Object)
}

func (s *CacheServer) Pull(
	ctx context.Context,
	req *types.PullRequest,
) (*types.CacheObject, error) {
	return s.storageProvider.Get(ctx, req.Key)
}

func (s *CacheServer) Query(
	ctx context.Context,
	req *types.QueryRequest,
) (*types.QueryResponse, error) {
	resp, err := s.storageProvider.Query(ctx, req.Keys)
	return &types.QueryResponse{
		Results: resp,
	}, err
}

func (s *CacheServer) Sync(*types.SyncRequest, types.Cache_SyncServer) error {
	return status.Errorf(codes.Unimplemented, "method Sync not implemented")
}
