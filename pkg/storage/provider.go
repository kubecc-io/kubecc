package storage

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
)

type StorageManager struct {
}

type StorageProvider interface {
	Location() types.StorageLocation
	Configure() error
	Put(context.Context, *types.CacheKey, *types.CacheObject) error
	Get(context.Context, *types.CacheKey) (*types.CacheObject, error)
	Query(context.Context, []*types.CacheKey) ([]*types.CacheObjectMeta, error)

	UsageInfo() *metrics.CacheUsage
	CacheHits() *metrics.CacheHits
}
