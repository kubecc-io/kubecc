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

package monitor

import (
	"context"
	"sync"

	"google.golang.org/protobuf/proto"
)

type KeyValueStore interface {
	Context() context.Context
	Set(key string, value proto.Message)
	Delete(key string)
	Get(key string) (proto.Message, bool)
	CAS(key string, value proto.Message) bool
	Keys() []string
	Len() int
}

type StoreCreator interface {
	NewStore(ctx context.Context) KeyValueStore
}

type InMemoryStore struct {
	data  map[string]proto.Message
	mutex *sync.RWMutex
	ctx   context.Context
}

type inMemoryStoreCreator struct{}

var InMemoryStoreCreator inMemoryStoreCreator

func (inMemoryStoreCreator) NewStore(ctx context.Context) KeyValueStore {
	return &InMemoryStore{
		data:  make(map[string]proto.Message),
		mutex: &sync.RWMutex{},
		ctx:   ctx,
	}
}

func (m *InMemoryStore) Context() context.Context {
	return m.ctx
}

func (m *InMemoryStore) Set(key string, value proto.Message) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[key] = value
}

func (m *InMemoryStore) Delete(key string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.data, key)
}

func (m *InMemoryStore) Get(key string) (proto.Message, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	data, ok := m.data[key]
	if ok {
		return proto.Clone(data), true
	}
	return nil, false
}

func (m *InMemoryStore) CAS(key string, value proto.Message) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	data, ok := m.data[key]
	if !ok || !proto.Equal(data, value) {
		m.data[key] = value
		return true
	}
	return false
}

func (m *InMemoryStore) Keys() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	keys := []string{}
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

func (m *InMemoryStore) Len() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.data)
}
