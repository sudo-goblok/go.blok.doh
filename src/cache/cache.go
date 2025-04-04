package cache

import (
	"sync"
	"time"
)

// CacheEntry menyimpan data hasil query dengan TTL
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// DNSTTLCache adalah cache dengan TTL untuk menyimpan hasil DNS query
type DNSTTLCache struct {
	store map[string]CacheEntry
	mu    sync.RWMutex
}

// NewDNSTTLCache membuat instance cache baru
func NewDNSTTLCache() *DNSTTLCache {
	return &DNSTTLCache{
		store: make(map[string]CacheEntry),
	}
}

// Get mengambil data dari cache jika belum kedaluwarsa
func (c *DNSTTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.store[key]
	if !found || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Data, true
}

// Set menyimpan data ke cache dengan TTL tertentu
func (c *DNSTTLCache) Set(key string, data interface{}, ttl uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}
}

// Cleanup menghapus entry cache yang kedaluwarsa
func (c *DNSTTLCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, v := range c.store {
		if now.After(v.ExpiresAt) {
			delete(c.store, k)
		}
	}
}

// StartCleanupLoop menjalankan proses pembersihan cache otomatis
func (c *DNSTTLCache) StartCleanupLoop(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			c.Cleanup()
		}
	}()
}
