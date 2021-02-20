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

type inMemoryStore struct {
	data  map[string][]byte
	mutex *sync.RWMutex
	sync.Map
}

func NewInMemoryStore(ctx context.Context) *inMemoryStore {
	return &inMemoryStore{
		data:  make(map[string][]byte),
		mutex: &sync.RWMutex{},
	}
}

func (m *inMemoryStore) Context() context.Context {
	return m.Context()
}

func (m *inMemoryStore) Set(key string, value []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[key] = value
}

func (m *inMemoryStore) Get(key string) ([]byte, bool) {
	m.mutex.RLock()
	m.mutex.RUnlock()
	data, ok := m.data[key]
	return data, ok
}

func (m *inMemoryStore) CAS(key string, value []byte) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	data, ok := m.data[key]
	if !ok {
		return false
	}
	if !bytes.Equal(data, value) {
		m.data[key] = value
		return true
	}
	return false
}
