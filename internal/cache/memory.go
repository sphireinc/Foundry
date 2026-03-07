package cache

import "sync"

type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]any
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]any),
	}
}

func (c *MemoryStore) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *MemoryStore) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *MemoryStore) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

func (c *MemoryStore) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]any)
}
