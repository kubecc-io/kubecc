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

// Package storage implements storage providers for the cache server.
package storage

import (
	"context"

	"github.com/cobalt77/kubecc/pkg/metrics"
	"github.com/cobalt77/kubecc/pkg/types"
)

// A StorageProvider represents an object capable of storing and retrieving
// cached data.
type StorageProvider interface {
	// Location should return the location kind of stored data.
	Location() types.StorageLocation
	// Configure should perform any necessary setup procedures that must be
	// completed prior to querying this storage provider.
	Configure() error
	// Put should store the given keyed object.
	Put(context.Context, *types.CacheKey, *types.CacheObject) error
	// Get should return the keyed object if it exists, or return
	// a relevant error if it does not exist.
	Get(context.Context, *types.CacheKey) (*types.CacheObject, error)
	// Query should return object metadata for each key in the provided
	// slice. If any key does not exist, the corresponding element in the
	// resulting slice should be nil. The length of the resulting slice
	// must match exactly with the length of the input slice.
	Query(context.Context, []*types.CacheKey) ([]*types.CacheObjectMeta, error)

	// UsageInfo should calculate and return usage information for the storage
	// provider.
	UsageInfo() *metrics.CacheUsage
	// CacheHits should return information about cache hits and misses.
	CacheHits() *metrics.CacheHits
}
