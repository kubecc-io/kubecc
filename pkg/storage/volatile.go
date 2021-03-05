package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cobalt77/kubecc/pkg/apps/cachesrv/metrics"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/karlseguin/ccache/v2"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
)

type volatileStorageProvider struct {
	cache            *ccache.Cache
	cfg              config.LocalStorageSpec
	storageLimit     int64
	totalSize        *atomic.Int64
	cacheHitsTotal   *atomic.Int64
	cacheMissesTotal *atomic.Int64
}

func NewVolatileStorageProvider(
	ctx context.Context,
	cfg config.LocalStorageSpec,
) (StorageProvider, error) {
	q, err := resource.ParseQuantity(cfg.Limits.Memory)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ConfigurationError, err.Error())
	}
	storageLimit := q.Value()
	totalSize := atomic.NewInt64(0)
	conf := ccache.Configure().
		MaxSize(storageLimit).
		ItemsToPrune(20).
		OnDelete(func(item *ccache.Item) {
			totalSize.Sub(item.Value().(*types.CacheObject).
				GetMetadata().
				GetManagedFields().
				GetSize())
		})

	sp := &volatileStorageProvider{
		cache:            ccache.New(conf),
		cfg:              cfg,
		storageLimit:     storageLimit,
		totalSize:        totalSize,
		cacheHitsTotal:   atomic.NewInt64(0),
		cacheMissesTotal: atomic.NewInt64(0),
	}
	return sp, nil
}

func (sp *volatileStorageProvider) Location() types.StorageLocation {
	return types.Memory
}

func (sp *volatileStorageProvider) Configure() error {
	return nil
}

func (sp *volatileStorageProvider) Put(
	ctx context.Context,
	key *types.CacheKey,
	object *types.CacheObject,
) error {
	if item := sp.cache.Get(key.GetHash()); item != nil {
		return status.Error(codes.AlreadyExists, "Object already exists")
	}
	sz := int64(len(object.Data))
	sp.totalSize.Add(sz)
	if object.Metadata == nil {
		object.Metadata = &types.CacheObjectMeta{}
	}
	object.Metadata.ManagedFields = &types.CacheObjectManaged{
		Location:  sp.Location(),
		Size:      sz,
		Timestamp: time.Now().Unix(),
	}
	sp.cache.Set(key.GetHash(), object,
		time.Until(time.Unix(0, object.Metadata.ExpirationTime)))
	return nil
}

func (sp *volatileStorageProvider) Get(
	ctx context.Context,
	key *types.CacheKey,
) (*types.CacheObject, error) {
	item := sp.cache.Get(key.GetHash())
	if item == nil {
		sp.cacheMissesTotal.Inc()
		return nil, status.Error(codes.NotFound, "Object not found")
	}
	obj := item.Value().(*types.CacheObject)
	if item.Expired() && obj.GetMetadata().GetExpirationTime() > 0 {
		sp.cache.Delete(key.GetHash())
		return nil, status.Error(codes.NotFound, "Object expired")
	}
	item.Extend(time.Until(time.Unix(0, obj.Metadata.ExpirationTime)))
	sp.cacheHitsTotal.Inc()
	return obj, nil
}

func (sp *volatileStorageProvider) Query(
	ctx context.Context,
	keys []*types.CacheKey,
) ([]*types.CacheObjectMeta, error) {
	results := make([]*types.CacheObjectMeta, len(keys))
	for i, key := range keys {
		if item := sp.cache.Get(key.GetHash()); item != nil {
			obj := item.Value().(*types.CacheObject)
			if item.Expired() && obj.GetMetadata().GetExpirationTime() > 0 {
				sp.cache.Delete(key.GetHash())
			} else {
				results[i] = item.Value().(*types.CacheObject).GetMetadata()
			}
		}
	}
	return results, nil
}

func (sp *volatileStorageProvider) UsageInfo() *metrics.UsageInfo {
	totalSize := sp.totalSize.Load()
	var usagePercent float64
	if sp.storageLimit == 0 {
		usagePercent = 0
	} else {
		usagePercent = float64(totalSize) / float64(sp.storageLimit)
	}
	return &metrics.UsageInfo{
		ObjectCount:  int64(sp.cache.ItemCount()),
		TotalSize:    totalSize,
		UsagePercent: usagePercent,
	}
}

func (sp *volatileStorageProvider) CacheHits() *metrics.CacheHits {
	hitTotal := sp.cacheHitsTotal.Load()
	missTotal := sp.cacheMissesTotal.Load()
	var percent float64
	if hitTotal+missTotal == 0 {
		percent = 0
	} else {
		percent = float64(hitTotal) / float64(hitTotal+missTotal)
	}
	return &metrics.CacheHits{
		CacheHitsTotal:   hitTotal,
		CacheMissesTotal: missTotal,
		CacheHitPercent:  percent,
	}
}
