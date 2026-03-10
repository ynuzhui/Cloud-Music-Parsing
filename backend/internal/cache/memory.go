package cache

import (
	"context"
	"sync"
	"time"
)

type memoryItem struct {
	Value    string
	ExpireAt time.Time
}

type MemoryCache struct {
	mu   sync.RWMutex
	data map[string]memoryItem
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{data: make(map[string]memoryItem)}
}

func (m *MemoryCache) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.RLock()
	item, ok := m.data[key]
	m.mu.RUnlock()
	if !ok {
		return "", false, nil
	}
	if time.Now().After(item.ExpireAt) {
		m.mu.Lock()
		delete(m.data, key)
		m.mu.Unlock()
		return "", false, nil
	}
	return item.Value, true, nil
}

func (m *MemoryCache) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	m.mu.Lock()
	m.data[key] = memoryItem{
		Value:    value,
		ExpireAt: time.Now().Add(ttl),
	}
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) Close() error {
	return nil
}
