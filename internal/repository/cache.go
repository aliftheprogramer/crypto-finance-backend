package repository

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

type Cache struct {
	mu    sync.RWMutex
	items map[string]*cacheEntry
	ttl   time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		items: make(map[string]*cacheEntry),
		ttl:   ttl,
	}
	go c.cleanup()
	return c
}

func (c *Cache) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.data
}

func (c *Cache) Set(key string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *Cache) cleanup() {
	for {
		time.Sleep(c.ttl)
		c.mu.Lock()
		for k, entry := range c.items {
			if time.Now().After(entry.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
