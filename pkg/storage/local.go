package storage

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/kubecc-io/kubecc/pkg/config"
	"github.com/kubecc-io/kubecc/pkg/metrics"
	"github.com/kubecc-io/kubecc/pkg/types"
	"go.uber.org/atomic"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Almost all of the business logic in this file was synthesized by Copilot!

// This file contains the logic for the "local" storage driver, which
// is a simple directory on disk that holds all blobs.

type localStorageProvider struct {
	ctx                context.Context
	root               string
	numObjects         int64
	totalSize          int64
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
	return &localStorageProvider{
		ctx:                ctx,
		numObjects:         0,
		totalSize:          0,
		root:               cfg.Path,
		sizeLimit:          limit.Value(),
		cacheHitsTotal:     atomic.NewInt64(0),
		cacheMissesTotal:   atomic.NewInt64(0),
		expirationNotifier: NewExpirationNotifier(),
	}
}

func (p *localStorageProvider) Location() types.StorageLocation {
	return types.Disk
}

func (p *localStorageProvider) Configure() error {
	// Ensure that the root directory exists.
	if err := os.MkdirAll(p.root, 0755); err != nil {
		return err
	}

	// Read all existing objects from disk and store the object count and total
	// size of all objects.
	p.numObjects = 0
	p.totalSize = 0
	if err := filepath.Walk(p.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		p.numObjects++
		p.totalSize += info.Size()

		// Unmarshal the object to get the expiration date.
		object := &types.CacheObject{}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if err := proto.Unmarshal(data, object); err != nil {
			return err
		}
		// Get the object's hash from the path.
		hash := filepath.Base(path)
		// Add the object to the expiration notifier.
		p.expirationNotifier.Add(hash, time.Unix(0, object.Metadata.GetExpirationDate()))
		return nil
	}); err != nil {
		return err
	}

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
				path := p.root + "/" + hash[0:2] + "/" + hash[2:]
				info, err := os.Stat(path)
				if err != nil {
					continue
				}
				p.totalSize -= info.Size()
				if err := os.Remove(path); err != nil {
					continue
				}
				p.numObjects--
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
				if p.totalSize > p.sizeLimit {
					p.DeleteObjectsClosestToExpiration()
				}
			}
		}
	}()
	return nil
}

func (p *localStorageProvider) DeleteObjectsClosestToExpiration() {
	// Until the total size is less than 90% of the size limit, trigger the
	// deletion of objects via the expiration notifier.
	for float64(p.totalSize) > float64(p.sizeLimit)*0.9 {
		// Force the notifier to expire the object it is currently monitoring.
		p.expirationNotifier.ForceExpire()
	}
}

func (p *localStorageProvider) Put(
	ctx context.Context,
	key *types.CacheKey,
	object *types.CacheObject,
) error {
	// Store objects as flat files in the directory named by the object hash
	// (which is the object's cache key).	Use a two level directory structure
	// to avoid too many files in a single directory. Objects are stored in the
	// protobuf binary format.
	objHash := key.Hash
	path := p.root + "/" + objHash[0:2] + "/" + objHash[2:]
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	data, err := proto.Marshal(object)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(path+"/"+objHash, data, 0644); err != nil {
		return err
	}
	return nil
}

func (p *localStorageProvider) Get(
	ctx context.Context,
	key *types.CacheKey,
) (*types.CacheObject, error) {
	// Check if the object exists on disk based on the provided hash and return
	// it if it does. Otherwise, return an error. The objects are stored in the
	// protobuf binary format.
	objHash := key.Hash
	path := p.root + "/" + objHash[0:2] + "/" + objHash[2:]
	data, err := ioutil.ReadFile(path + "/" + objHash)
	if err != nil {
		return nil, err
	}
	object := &types.CacheObject{}
	if err := proto.Unmarshal(data, object); err != nil {
		return nil, err
	}
	// Update the object's timestamp to the current time.
	object.Metadata.ManagedFields.Timestamp = time.Now().UnixNano()
	return object, nil
}

func (p *localStorageProvider) Query(
	ctx context.Context,
	keys []*types.CacheKey,
) ([]*types.CacheObjectMeta, error) {
	// Query the local storage provider for the objects that match the provided
	// keys. The objects are stored in the protobuf binary format.
	var objects []*types.CacheObjectMeta
	for _, key := range keys {
		objHash := key.Hash
		path := p.root + "/" + objHash[0:2] + "/" + objHash[2:]
		data, err := ioutil.ReadFile(path + "/" + objHash)
		if err != nil {
			return nil, err
		}
		object := &types.CacheObject{}
		if err := proto.Unmarshal(data, object); err != nil {
			return nil, err
		}
		objects = append(objects, &types.CacheObjectMeta{
			Tags:           object.Metadata.GetTags(),
			ExpirationDate: object.Metadata.GetExpirationDate(),
			ManagedFields:  object.Metadata.GetManagedFields(),
		})
	}
	return objects, nil
}

func (p *localStorageProvider) UsageInfo() *metrics.CacheUsage {
	return &metrics.CacheUsage{
		ObjectCount: p.numObjects,
		TotalSize:   p.totalSize,
	}
}

func (p *localStorageProvider) CacheHits() *metrics.CacheHits {
	panic("not implemented") // TODO: Implement
}

// ExpirationNotifier provides an efficient way to monitor the expiration of
// many objects. It stores expiration dates of objects in a binary heap and
// only waits for the next expiration event. It is used by the localStorageMonitor
// to monitor the expiration of objects.
type ExpirationNotifier struct {
	// The heap is a binary heap that stores the expiration dates of objects.
	// The heap is indexed by the hash of the object.
	heap *ExpirationHeap
	// The lock protects the heap.
	lock sync.Mutex

	// The channel that is used to signal the expiration of an object.
	// The channel is buffered to prevent the expiration of objects
	// that are currently being added to the heap.
	expirationChan chan string

	// The channel that is used to tell the waiter to rescan the heap when
	// an object is added to the heap with an expiration date that is
	// earlier than the expiration date of the object currently being waited on.
	rescanChan chan bool

	// The channel that is used to wake the waiter when a new object is added
	// to the heap.
	addChan chan string

	// The channel that is used to force the expiration of the current object
	// being waited on.
	forceExpireChan chan struct{}
}

func NewExpirationNotifier() *ExpirationNotifier {
	return &ExpirationNotifier{
		heap:           NewExpirationHeap(),
		expirationChan: make(chan string, 1000),
		rescanChan:     make(chan bool),
		addChan:        make(chan string),
	}
}

func (e *ExpirationNotifier) Add(hash string, expirationDate time.Time) {
	e.lock.Lock()
	defer e.lock.Unlock()
	// Add the object to the heap.
	e.heap.Push(hash, expirationDate)

	// If the new object is the first object in the heap, then we need to
	// tell the waiter to rescan the heap.
	if h, _ := e.heap.Peek(); h == hash {
		select {
		case e.rescanChan <- true:
		default:
		}
	}

	// If the new object is the first object in the heap, then we may need to
	// wake the waiter.
	if e.heap.Len() == 1 {
		select {
		case e.addChan <- hash:
		default:
		}
	}
}

func (e *ExpirationNotifier) NextExpiration() (string, time.Time) {
	e.lock.Lock()
	defer e.lock.Unlock()
	// Get the next expiration event.
	hash, expirationDate := e.heap.Peek()
	return hash, expirationDate
}

func (e *ExpirationNotifier) WaitForExpiration(ctx context.Context) {
	for {
		// Get the next expiration event.
		hash, expirationDate := e.NextExpiration()
		if hash == "" && expirationDate.IsZero() {
			// The heap is empty.
			return
		}

		// Wait for the next expiration event.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(expirationDate)):
			e.expirationChan <- hash
			// Pop the object from the heap.
			e.lock.Lock()
			e.heap.Pop()
			e.lock.Unlock()
		case <-e.forceExpireChan:
			e.expirationChan <- hash
			// Pop the object from the heap.
			e.lock.Lock()
			e.heap.Pop()
			e.lock.Unlock()
		case <-e.rescanChan:
			// Rescan the heap for the latest expiration event.
		}
	}
}

func (e *ExpirationNotifier) Monitor(ctx context.Context) {
	// Monitor the expiration of objects.
	for {
		e.WaitForExpiration(ctx)
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

func (e *ExpirationNotifier) ForceExpire() {
	// Force the expiration of the current object.
	e.lock.Lock()
	defer e.lock.Unlock()
	select {
	case e.forceExpireChan <- struct{}{}:
	default:
	}
}

// ExpirationHeap implements a heap that stores the expiration dates of objects.
// The heap is indexed by the hash of the object.
type ExpirationHeap struct {
	// The heap is a binary heap that stores the expiration dates of objects.
	// The heap is indexed by the hash of the object.
	heap []*ExpirationEntry
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

func (h *ExpirationHeap) heapify() {
	for i := len(h.heap) / 2; i >= 0; i-- {
		h.siftDown(i)
	}
}

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

func (h *ExpirationHeap) swap(m, n int) {
	h.heap[m], h.heap[n] = h.heap[n], h.heap[m]
}

// Push adds a new object to the heap.
func (h *ExpirationHeap) Push(hash string, expirationDate time.Time) {
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
	if len(h.heap) == 0 {
		return "", time.Time{}
	}
	return h.heap[0].Hash, h.heap[0].Date
}

// Len returns the number of objects in the heap.
func (h *ExpirationHeap) Len() int {
	return len(h.heap)
}

// Push adds a new object to the heap.
func (h *ExpirationHeap) Pop() *ExpirationEntry {
	if len(h.heap) == 0 {
		return nil
	}
	e := h.heap[0]
	h.heap[0] = h.heap[len(h.heap)-1]
	h.heap = h.heap[:len(h.heap)-1]
	h.heapify()
	return e
}

func (h *ExpirationHeap) UnderlyingArray() []*ExpirationEntry {
	return h.heap
}
