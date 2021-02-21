package monitor

import (
	"bytes"
	"context"
	"sync"
)

type KeyValueStore interface {
	Context() context.Context
	Set(key string, value []byte)
	Get(key string) ([]byte, bool)
	CAS(key string, value []byte) bool
}

type StoreCreator interface {
	NewStore(ctx context.Context) KeyValueStore
}

type InMemoryStore struct {
	data  map[string][]byte
	mutex *sync.RWMutex
	ctx   context.Context
	sync.Map
}

type inMemoryStoreCreator struct{}

var InMemoryStoreCreator inMemoryStoreCreator

func (inMemoryStoreCreator) NewStore(ctx context.Context) KeyValueStore {
	return &InMemoryStore{
		data:  make(map[string][]byte),
		mutex: &sync.RWMutex{},
		ctx:   ctx,
	}
}

func (m *InMemoryStore) Context() context.Context {
	return m.ctx
}

func (m *InMemoryStore) Set(key string, value []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[key] = value
}

func (m *InMemoryStore) Get(key string) ([]byte, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	data, ok := m.data[key]
	return data, ok
}

func (m *InMemoryStore) CAS(key string, value []byte) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	data, ok := m.data[key]
	if !ok || !bytes.Equal(data, value) {
		m.data[key] = value
		return true
	}
	return false
}
