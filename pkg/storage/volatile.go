package storage

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/apps/cachesrv/metrics"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
)

type volatileStorageProvider struct {
	cache             map[string]*types.CacheObject
	cacheMutex        *sync.RWMutex
	totalSize         *atomic.Int64
	cfg               config.LocalStorageSpec
	storageLimitBytes int64
	cacheHitsTotal    *atomic.Int64
	cacheMissesTotal  *atomic.Int64
}

func NewVolatileStorageProvider(
	ctx context.Context,
	cfg config.LocalStorageSpec,
) StorageProvider {
	sp := &volatileStorageProvider{
		cache:            make(map[string]*types.CacheObject),
		cfg:              cfg,
		cacheMutex:       &sync.RWMutex{},
		totalSize:        atomic.NewInt64(0),
		cacheHitsTotal:   atomic.NewInt64(0),
		cacheMissesTotal: atomic.NewInt64(0),
	}
	q, err := resource.ParseQuantity(sp.cfg.Limits.Memory)
	if err != nil {
		meta.Log(ctx).Fatal("%w: %s", ConfigurationError, err)
	}
	sp.storageLimitBytes = q.Value()
	return sp
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
	sp.cacheMutex.Lock()
	defer sp.cacheMutex.Unlock()

	if _, ok := sp.cache[key.GetHash()]; ok {
		return status.Error(codes.AlreadyExists, "Object already exists")
	}
	sp.totalSize.Add(int64(len(object.Data)))
	sp.cache[key.GetHash()] = object
	return nil
}

func (sp *volatileStorageProvider) Get(
	ctx context.Context,
	key *types.CacheKey,
) (*types.CacheObject, error) {
	sp.cacheMutex.RLock()
	defer sp.cacheMutex.RUnlock()

	obj, ok := sp.cache[key.GetHash()]
	if !ok {
		sp.cacheMissesTotal.Inc()
		return nil, status.Error(codes.NotFound, "Object not found")
	}
	sp.cacheHitsTotal.Inc()
	return obj, nil
}

func (sp *volatileStorageProvider) Query(
	ctx context.Context,
	keys []*types.CacheKey,
) ([]*types.CacheObjectMeta, error) {
	sp.cacheMutex.RLock()
	defer sp.cacheMutex.RUnlock()

	results := make([]*types.CacheObjectMeta, len(keys))
	for i, key := range keys {
		if obj, ok := sp.cache[key.GetHash()]; ok {
			results[i] = obj.GetMetadata()
		}
	}
	return results, nil
}

func (sp *volatileStorageProvider) UsageInfo() *metrics.UsageInfo {
	sp.cacheMutex.RLock()
	defer sp.cacheMutex.RUnlock()

	totalSize := sp.totalSize.Load()
	var usagePercent float64
	if sp.storageLimitBytes == 0 {
		usagePercent = 0
	} else {
		usagePercent = float64(totalSize) / float64(sp.storageLimitBytes)
	}
	return &metrics.UsageInfo{
		ObjectCount:  int64(len(sp.cache)),
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
