package storage

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Almost all of the business logic in this file was synthesized by Copilot!

// This file contains the logic for the "local" storage driver, which
// is a simple directory on disk that holds all blobs.

type LocalStorageProvider struct {
	ctx                context.Context
	lg                 *zap.SugaredLogger
	root               string
	numObjects         *atomic.Int64
	totalSize          *atomic.Int64
	sizeLimit          int64
	cacheHitsTotal     *atomic.Int64
	cacheMissesTotal   *atomic.Int64
	expirationNotifier *ExpirationNotifier
}

func NewLocalStorageProvider(
	ctx context.Context,
	cfg config.LocalStorageSpec,
) StorageProvider {
	// Parse size limit from the configuration and convert it to bytes.
	limit := resource.MustParse(cfg.Limits.Disk)
	return &LocalStorageProvider{
		ctx:                ctx,
		lg:                 meta.Log(ctx),
		numObjects:         atomic.NewInt64(0),
		totalSize:          atomic.NewInt64(0),
		root:               cfg.Path,
		sizeLimit:          limit.Value(),
		cacheHitsTotal:     atomic.NewInt64(0),
		cacheMissesTotal:   atomic.NewInt64(0),
		expirationNotifier: NewExpirationNotifier(),
	}
}

var ForceExpirationFailedErr = errors.New("failed to force expiration of current object")

func (p *LocalStorageProvider) Location() types.StorageLocation {
	return types.Disk
}

func (p *LocalStorageProvider) Configure() error {
	// Ensure that the root directory exists.
	if err := os.MkdirAll(p.root, 0755); err != nil {
		return err
	}

	// Read all existing objects from disk and store the object count and total
	// size of all objects.
	p.numObjects.Store(0)
	p.totalSize.Store(0)
	toDelete := []string{}
	if err := filepath.Walk(p.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		p.numObjects.Inc()
		p.totalSize.Add(info.Size())

		// Unmarshal the object to get the expiration date.
		object := &types.CacheObject{}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if err := proto.Unmarshal(data, object); err != nil {
			p.lg.With(
				zap.String("path", path),
				zap.Error(err),
			).Error("failed to unmarshal object, deleting it from the cache")
			toDelete = append(toDelete, path)
			return nil
		}
		// Get the object's hash from the path.
		hash := filepath.Base(path)
		// Add the object to the expiration notifier.
		p.expirationNotifier.Add(hash, time.Unix(0, object.Metadata.GetExpirationDate()))
		return nil
	}); err != nil {
		return err
	}

	// Delete the objects that failed to unmarshal.
	for _, path := range toDelete {
		if err := os.Remove(path); err != nil {
			// Log the error, but don't exit.
			p.lg.With(
				zap.String("path", path),
				zap.Error(err),
			).Error("failed to delete object from the cache")
		}
	}

	// Log the current state of the cache.
	p.lg.With(
		"objects", p.numObjects.Load(),
		"sizeMB", p.totalSize.Load()/1024/1024,
		"limitMB", p.sizeLimit/1024/1024,
	).Info("Local cache initialized")

	// Start the expiration notifier.
	go p.expirationNotifier.Monitor(p.ctx)

	// Set up a goroutine to process expiration notifications.
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case hash := <-p.expirationNotifier.Notify():
				// Read the object's size and delete it.
				path := path.Join(p.root, hash[0:2], hash)
				info, err := os.Stat(path)
				if err != nil {
					p.lg.Error(err)
					continue
				}
				if err := os.Remove(path); err != nil {
					p.lg.With(
						zap.String("hash", hash),
					).Error("failed to delete expired object", zap.Error(err))
					continue
				}
				p.totalSize.Sub(info.Size())
				p.numObjects.Dec()
			}
		}
	}()

	// Set up a goroutine to delete old objects if the total size exceeds the
	// size limit.
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-time.After(time.Minute):
				if p.totalSize.Load() > p.sizeLimit {
					p.DeleteObjectsClosestToExpiration()
				}
			}
		}
	}()
	return nil
}

func (p *LocalStorageProvider) DeleteObjectsClosestToExpiration() {
	p.lg.Info("Deleting objects closest to expiration")

	// Keep track of the total number of objects deleted so we can log it
	// at the end.
	numDeleted := 0

	// Until the total size is less than 90% of the size limit, trigger the
	// deletion of objects via the expiration notifier.
	for float64(p.totalSize.Load()) >= float64(p.sizeLimit)*0.9 {
		// Get the next object which is closest to expiring.
		hash, _ := p.expirationNotifier.heap.Peek()

		// Force the notifier to expire the object it is currently monitoring.
		if err := p.expirationNotifier.ForceExpiration(); err != nil {
			// If the expiration failed, then we should not continue.
			p.lg.Error(err)
			return
		}
		// Increment the number of objects deleted.
		numDeleted++

		// Wait until the object is expired
		err := wait.Poll(100*time.Millisecond, 2*time.Second, func() (bool, error) {
			// Try to get the object with the hash that we just forced to expire.
			// If the object is not found, then it has been deleted.
			_, err := p.Get(p.ctx, &types.CacheKey{
				Hash: hash,
			})
			if err != nil {
				// If the object is not found, then we can continue.
				return true, nil
			}
			// The object is still in the cache, so we need to wait for it to
			// expire.
			return false, nil
		})
		if err != nil {
			p.lg.With(
				zap.String("hash", hash),
			).Error("failed to wait for object to expire", zap.Error(err))
			return
		}
	}

	// Log the number of objects deleted.
	p.lg.With(
		"count", numDeleted,
	).Info("Deleted objects to reclaim disk space")
}

func (p *LocalStorageProvider) Put(
	ctx context.Context,
	key *types.CacheKey,
	object *types.CacheObject,
) error {
	lg := meta.Log(ctx)
	// Fill in the object's managed fields
	object.Metadata.ManagedFields = &types.CacheObjectManaged{
		Size:      int64(len(object.Data)),
		Timestamp: time.Now().UnixNano(),
		Location:  types.Disk,
	}

	lg.With(
		"size", fmt.Sprintf("%d Ki", len(object.Data)/1024),
		"tags", object.Metadata.GetTags(),
		"ttl", time.Until(time.UnixMicro(object.Metadata.GetExpirationDate())),
	).Info("Storing new object")

	// Store objects as flat files in the directory named by the object hash
	// (which is the object's cache key).	Use a two level directory structure
	// to avoid too many files in a single directory. Objects are stored in the
	// protobuf binary format.
	objHash := key.Hash
	objPath := path.Join(p.root, objHash[0:2])
	if err := os.MkdirAll(objPath, 0755); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	data, err := proto.Marshal(object)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if err := ioutil.WriteFile(path.Join(objPath, objHash), data, 0644); err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	// Add the object's size to the total size.
	p.totalSize.Add(int64(len(object.Data)))
	p.numObjects.Inc()

	// Add the object to the expiration notifier.
	p.expirationNotifier.Add(objHash, time.Unix(0, object.Metadata.GetExpirationDate()))
	return nil
}

func (p *LocalStorageProvider) Get(
	ctx context.Context,
	key *types.CacheKey,
) (*types.CacheObject, error) {
	// Check if the object exists on disk based on the provided hash and return
	// it if it does. Otherwise, return an error. The objects are stored in the
	// protobuf binary format.
	objHash := key.Hash
	objPath := path.Join(p.root, objHash[0:2], objHash)
	if _, err := os.Stat(objPath); err != nil {
		// Increment cache misses.
		p.cacheMissesTotal.Inc()
		return nil, status.Error(codes.NotFound,
			fmt.Errorf("Object not found: %w", err).Error())
	}
	p.cacheHitsTotal.Inc()
	// Read the object from disk.
	data, err := ioutil.ReadFile(objPath)
	if err != nil {
		// Something went wrong reading the object, but it exists.
		return nil, status.Error(codes.NotFound,
			fmt.Errorf("Error retrieving object: %w", err).Error())
	}
	// Unmarshal the object from the data.
	object := &types.CacheObject{}
	if err := proto.Unmarshal(data, object); err != nil {
		return nil, status.Error(codes.NotFound,
			fmt.Errorf("Object is corrupted or invalid: %w", err).Error())
	}

	// Fill in some fields that are set to omitempty
	if object.Metadata == nil {
		object.Metadata = &types.CacheObjectMeta{}
	}
	if object.Metadata.Tags == nil {
		object.Metadata.Tags = make(map[string]string)
	}
	// Update the object's timestamp to the current time.
	object.Metadata.ManagedFields.Timestamp = time.Now().UnixNano()
	return object, nil
}

func (p *LocalStorageProvider) Query(
	ctx context.Context,
	keys []*types.CacheKey,
) ([]*types.CacheObjectMeta, error) {
	// Query the local storage provider for the objects that match the provided
	// keys. The objects are stored in the protobuf binary format.
	var objects []*types.CacheObjectMeta
	for _, key := range keys {
		objHash := key.Hash
		objPath := path.Join(p.root, objHash[0:2], objHash)
		if _, err := os.Stat(objPath); err != nil {
			continue
		}
		// Read the object from disk.
		data, err := ioutil.ReadFile(objPath)
		if err != nil {
			p.lg.Debug(err)
			continue
		}
		// Unmarshal the object from the data.
		object := &types.CacheObject{}
		if err := proto.Unmarshal(data, object); err != nil {
			p.lg.Debug(err)
			continue
		}
		// Fill in some fields that are set to omitempty
		if object.Metadata == nil {
			object.Metadata = &types.CacheObjectMeta{}
		}
		if object.Metadata.Tags == nil {
			object.Metadata.Tags = make(map[string]string)
		}
		objects = append(objects, &types.CacheObjectMeta{
			Tags:           object.Metadata.GetTags(),
			ExpirationDate: object.Metadata.GetExpirationDate(),
			ManagedFields:  object.Metadata.GetManagedFields(),
		})
	}
	return objects, nil
}

func (p *LocalStorageProvider) UsageInfo() *metrics.CacheUsage {
	count := p.numObjects.Load()
	size := p.totalSize.Load()
	return &metrics.CacheUsage{
		ObjectCount:  count,
		TotalSize:    size,
		UsagePercent: float64(size) / float64(p.sizeLimit) * 100,
	}
}

func (p *LocalStorageProvider) CacheHits() *metrics.CacheHits {
	// Load the number of cache hits and misses, and calculate the hit percentage.
	hits := p.cacheHitsTotal.Load()
	misses := p.cacheMissesTotal.Load()
	total := hits + misses
	if total > 0 {
		return &metrics.CacheHits{
			CacheHitsTotal:   hits,
			CacheMissesTotal: misses,
			CacheHitPercent:  float64(hits) / float64(total),
		}
	}
	return nil
}

// ExpirationNotifier provides an efficient way to monitor the expiration of
// many objects. It stores expiration dates of objects in a binary heap and
// only waits for the next expiration event. It is used by the localStorageMonitor
// to monitor the expiration of objects.
type ExpirationNotifier struct {
	// The heap is a binary heap that stores the expiration dates of objects.
	// The heap is indexed by the hash of the object.
	heap *ExpirationHeap

	// The channel that is used to signal the expiration of an object.
	// The channel is buffered to prevent the expiration of objects
	// that are currently being added to the heap.
	expirationChan chan string

	// The channel that is used to tell the waiter to rescan the heap when
	// an object is added to the heap with an expiration date that is
	// earlier than the expiration date of the object currently being waited on.
	rescanChan chan struct{}

	// The channel that is used to wake the waiter when a new object is added
	// to the heap.
	addChan chan struct{}

	// The channel that is used to force the expiration of the current object
	// being waited on.
	forceExpireChan chan struct{}
}

func NewExpirationNotifier() *ExpirationNotifier {
	return &ExpirationNotifier{
		heap:            NewExpirationHeap(),
		expirationChan:  make(chan string, 1000),
		rescanChan:      make(chan struct{}, 1),
		addChan:         make(chan struct{}, 1),
		forceExpireChan: make(chan struct{}),
	}
}

func (e *ExpirationNotifier) Add(hash string, expirationDate time.Time) {
	// Add the object to the heap.
	e.heap.Push(hash, expirationDate)

	// If the new object is the first object in the heap, then we need to
	// tell the waiter to rescan the heap.
	if h, _ := e.heap.Peek(); h == hash {
		select {
		case e.rescanChan <- struct{}{}:
		default:
		}
	}

	// If the new object is the first object in the heap, then we may need to
	// wake the waiter.
	if e.heap.Len() == 1 {
		select {
		case e.addChan <- struct{}{}:
		default:
		}
	}
}

func (e *ExpirationNotifier) NextExpiration() (string, time.Time) {
	// Get the next expiration event.
	hash, expirationDate := e.heap.Peek()
	return hash, expirationDate
}

func (e *ExpirationNotifier) WaitOne(ctx context.Context) {
rescan:
	if e.heap.Len() == 0 {
		// The heap is empty.
		return
	}
	// Get the next expiration event.
	hash, expirationDate := e.NextExpiration()

	// Wait for the next expiration event.
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Until(expirationDate)):
		// Pop the object from the heap.
		e.heap.Pop()
		e.expirationChan <- hash
	case <-e.forceExpireChan:
		// Pop the object from the heap.
		e.heap.Pop()
		e.expirationChan <- hash
	case <-e.rescanChan:
		// Rescan the heap for the latest expiration event.
		goto rescan
	}
}

func (e *ExpirationNotifier) WaitAll(ctx context.Context) {
	for ctx.Err() == nil && e.heap.Len() > 0 {
		e.WaitOne(ctx)
	}
}

func (e *ExpirationNotifier) Monitor(ctx context.Context) {
	// Monitor the expiration of objects.
	for {
		e.WaitAll(ctx)
		// No more objects to monitor. Wait until we are woken up by a new object
		// being added to the heap.
		select {
		case <-ctx.Done():
			return
		case <-e.addChan:
		}
	}
}

func (e *ExpirationNotifier) Notify() <-chan string {
	return e.expirationChan
}

func (e *ExpirationNotifier) ForceExpiration() error {
	// Force the expiration of the current object.
	select {
	case e.forceExpireChan <- struct{}{}:
		return nil
	case <-time.After(time.Second):
		// If the forceExpireChan was not read from within 1 second, then
		// there was no object to force expiration.
		return ForceExpirationFailedErr
	}
}

// ExpirationHeap implements a heap that stores the expiration dates of objects.
// The heap is indexed by the hash of the object. The heap is thread-safe.
type ExpirationHeap struct {
	// The heap is a binary heap that stores the expiration dates of objects.
	// The heap is indexed by the hash of the object.
	heap []*ExpirationEntry

	// The lock protects the heap.
	lock sync.Mutex
}

// ExpirationEntry is an entry in the ExpirationHeap.
type ExpirationEntry struct {
	Hash string
	Date time.Time
}

// NewExpirationHeap creates a new ExpirationHeap.
func NewExpirationHeap() *ExpirationHeap {
	return &ExpirationHeap{
		heap: make([]*ExpirationEntry, 0, 1000),
	}
}

// The lock must be held when calling this function.
func (h *ExpirationHeap) heapify() {
	for i := len(h.heap) / 2; i >= 0; i-- {
		h.siftDown(i)
	}
}

// The lock must be held when calling this function.
func (h *ExpirationHeap) siftDown(i int) {
	for {
		l := 2*i + 1
		r := 2*i + 2
		m := i
		if l < len(h.heap) && h.heap[l].Date.Before(h.heap[m].Date) {
			m = l
		}
		if r < len(h.heap) && h.heap[r].Date.Before(h.heap[m].Date) {
			m = r
		}
		if m != i {
			h.swap(m, i)
			i = m
		} else {
			break
		}
	}
}

// The lock must be held when calling this function.
func (h *ExpirationHeap) swap(m, n int) {
	h.heap[m], h.heap[n] = h.heap[n], h.heap[m]
}

// Push adds a new object to the heap.
func (h *ExpirationHeap) Push(hash string, expirationDate time.Time) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.heap = append(h.heap, &ExpirationEntry{
		Hash: hash,
		Date: expirationDate,
	})

	// Move the new object to the top of the heap.
	h.swap(len(h.heap)-1, 0)
	h.heapify()
}

// Peek returns the next object in the heap.
func (h *ExpirationHeap) Peek() (string, time.Time) {
	h.lock.Lock()
	defer h.lock.Unlock()

	if len(h.heap) == 0 {
		return "", time.Time{}
	}
	return h.heap[0].Hash, h.heap[0].Date
}

// Len returns the number of objects in the heap.
func (h *ExpirationHeap) Len() int {
	h.lock.Lock()
	defer h.lock.Unlock()

	return len(h.heap)
}

// Push adds a new object to the heap.
func (h *ExpirationHeap) Pop() *ExpirationEntry {
	h.lock.Lock()
	defer h.lock.Unlock()

	if len(h.heap) == 0 {
		return nil
	}
	e := h.heap[0]
	h.heap[0] = h.heap[len(h.heap)-1]
	h.heap = h.heap[:len(h.heap)-1]
	h.heapify()
	return e
}

// UnderlyingArray returns a copy of the underlying array of the heap.
// This function should only be used for testing.
func (h *ExpirationHeap) UnderlyingArray() []*ExpirationEntry {
	h.lock.Lock()
	defer h.lock.Unlock()
	return append([]*ExpirationEntry{}, h.heap...)
}
