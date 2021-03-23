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
	"strings"
	"sync"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type ChainStorageProvider struct {
	ctx       context.Context
	lg        *zap.SugaredLogger
	providers []StorageProvider
}

func NewChainStorageProvider(
	ctx context.Context,
	providers ...StorageProvider,
) StorageProvider {
	sp := &ChainStorageProvider{
		ctx:       ctx,
		lg:        meta.Log(ctx),
		providers: providers,
	}
	return sp
}

func (sp *ChainStorageProvider) Location() types.StorageLocation {
	return types.Memory
}

func (sp *ChainStorageProvider) Configure() error {
	locations := []string{}
	for _, p := range sp.providers {
		if err := p.Configure(); err != nil {
			return err
		}
		locations = append(locations,
			strings.Replace(p.Location().String(), "StorageLocation_", "", 1))
	}
	sp.lg.Infof("Cache order: %s", strings.Join(locations, " => "))
	return nil
}

func (sp *ChainStorageProvider) Put(
	ctx context.Context,
	key *types.CacheKey,
	object *types.CacheObject,
) error {
	for _, p := range sp.providers {
		if err := p.Put(ctx, key, object); err != nil {
			return err
		}
	}
	return nil
}

func (sp *ChainStorageProvider) Get(
	ctx context.Context,
	key *types.CacheKey,
) (object *types.CacheObject, err error) {
	i := 0
	// Find the first provider containing the object
	for ; i < len(sp.providers); i++ {
		p := sp.providers[i]
		if object, err = p.Get(ctx, key); err == nil {
			// Found
			break
		}
	}

	if err != nil {
		// Not found anywhere
		return nil, err
	}

	// Loop backwards through providers missing the key and store them
	go func(k *types.CacheKey, obj *types.CacheObject) {
		for i--; i >= 0; i-- {
			p := sp.providers[i]
			if err := p.Put(sp.ctx, k, obj); err != nil {
				// Something bad happened
				sp.lg.Error(err)
				break
			}
		}
	}(
		proto.Clone(key).(*types.CacheKey),
		proto.Clone(object).(*types.CacheObject),
	)
	return
}

func (sp *ChainStorageProvider) Query(
	ctx context.Context,
	keys []*types.CacheKey,
) ([]*types.CacheObjectMeta, error) {
	results := make([]*types.CacheObjectMeta, len(keys))
	var wg sync.WaitGroup
	wg.Add(len(results))
	for i := range results {
		go func(i int) {
			defer wg.Done()
			for p := 0; p < len(sp.providers) && results[i] == nil; p++ {
				result, _ := sp.providers[i].Query(ctx, keys)
				results[i] = result[0]
			}
		}(i)
	}
	wg.Wait()
	return results, nil
}

func (sp *ChainStorageProvider) UsageInfo() *metrics.CacheUsage {
	return sp.providers[0].UsageInfo() // todo
}

func (sp *ChainStorageProvider) CacheHits() *metrics.CacheHits {
	return sp.providers[0].CacheHits() // todo
}
