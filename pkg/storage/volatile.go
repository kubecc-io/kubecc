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

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/karlseguin/ccache/v2"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
)

type volatileStorageProvider struct {
	ctx              context.Context
	lg               *zap.SugaredLogger
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
) StorageProvider {
	sp := &volatileStorageProvider{
		ctx:              ctx,
		lg:               meta.Log(ctx),
		cfg:              cfg,
		cacheHitsTotal:   atomic.NewInt64(0),
		cacheMissesTotal: atomic.NewInt64(0),
	}
	return sp
}

func (sp *volatileStorageProvider) Location() types.StorageLocation {
	return types.Memory
}

func (sp *volatileStorageProvider) Configure() error {
	q, err := resource.ParseQuantity(sp.cfg.Limits.Memory)
	if err != nil {
		return fmt.Errorf("%w: %s", ConfigurationError, err.Error())
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
	sp.cache = ccache.New(conf)
	sp.storageLimit = storageLimit
	sp.totalSize = totalSize
	sp.lg.Info("In-memory storage provider configured")
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
		time.Until(time.Unix(0, object.Metadata.ExpirationDate)))
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
	if item.Expired() && obj.GetMetadata().GetExpirationDate() > 0 {
		sp.cache.Delete(key.GetHash())
		return nil, status.Error(codes.NotFound, "Object expired")
	}
	item.Extend(time.Until(time.Unix(0, obj.Metadata.ExpirationDate)))
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
			if item.Expired() && obj.GetMetadata().GetExpirationDate() > 0 {
				sp.cache.Delete(key.GetHash())
			} else {
				results[i] = item.Value().(*types.CacheObject).GetMetadata()
			}
		}
	}
	return results, nil
}

func (sp *volatileStorageProvider) UsageInfo() *metrics.CacheUsage {
	totalSize := sp.totalSize.Load()
	var usagePercent float64
	if sp.storageLimit == 0 {
		usagePercent = 0
	} else {
		usagePercent = float64(totalSize) / float64(sp.storageLimit)
	}
	return &metrics.CacheUsage{
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
