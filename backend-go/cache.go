package main

import (
	"sync"
	"time"
)

type CacheItem struct {
	Value     string
	ExpiresAt time.Time
}

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]CacheItem
}

var globalCache = NewMemoryCache()

func NewMemoryCache() *MemoryCache {
	c := &MemoryCache{
		items: make(map[string]CacheItem),
	}
	go c.startCleaner()
	return c
}

func (c *MemoryCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return "", false
	}
	if time.Now().After(item.ExpiresAt) {
		return "", false
	}
	return item.Value, true
}

func (c *MemoryCache) Set(key string, val string, ttlSeconds int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttlSeconds <= 0 {
		ttlSeconds = 300 // default 5 minutes
	}

	c.items[key] = CacheItem{
		Value:     val,
		ExpiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	}
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]CacheItem)
}

func (c *MemoryCache) startCleaner() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.ExpiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
