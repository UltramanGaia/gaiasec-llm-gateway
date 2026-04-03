package handlers

import (
	"sync"
	"time"

	"llm-gateway/models"
)

type CacheItem struct {
	Value      interface{}
	Expiration int64
}

type MemoryCache struct {
	items map[string]*CacheItem
	mu    sync.RWMutex
}

var (
	globalCache          *MemoryCache
	cacheOnce            sync.Once
	modelConfigCache     = make(map[string]*CacheItem)
	modelConfigMu        sync.RWMutex
	modelConfigCacheOnce sync.Once
	modelConfigCleanupCh chan struct{}
	// Default TTL for model config cache - adjust as needed
	ModelConfigCacheTTL = 5 * time.Minute
)

func GetCache() *MemoryCache {
	cacheOnce.Do(func() {
		globalCache = &MemoryCache{
			items: make(map[string]*CacheItem),
		}
		go globalCache.cleanup()
	})
	return globalCache
}

func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &CacheItem{
		Value:      value,
		Expiration: time.Now().Add(ttl).UnixNano(),
	}
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, found := c.items[key]
	if !found {
		return nil, false
	}
	if time.Now().UnixNano() > item.Expiration {
		return nil, false
	}
	return item.Value, true
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now().UnixNano()
		for key, item := range c.items {
			if now > item.Expiration {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

func SetModelConfigs(name string, configs []models.ModelConfig) {
	SetModelConfigsWithTTL(name, configs, ModelConfigCacheTTL)
}

func SetModelConfigsWithTTL(name string, configs []models.ModelConfig, ttl time.Duration) {
	modelConfigMu.Lock()
	defer modelConfigMu.Unlock()
	copied := make([]models.ModelConfig, len(configs))
	copy(copied, configs)
	modelConfigCache[name] = &CacheItem{
		Value:      copied,
		Expiration: time.Now().Add(ttl).UnixNano(),
	}
	startModelConfigCleanup()
}

func GetModelConfigsFromCache(name string) ([]models.ModelConfig, bool) {
	modelConfigMu.RLock()
	defer modelConfigMu.RUnlock()
	item, found := modelConfigCache[name]
	if !found {
		return nil, false
	}
	if time.Now().UnixNano() > item.Expiration {
		return nil, false
	}
	config := item.Value.([]models.ModelConfig)
	copied := make([]models.ModelConfig, len(config))
	copy(copied, config)
	return copied, true
}

func InvalidateModelConfigCache(name string) {
	modelConfigMu.Lock()
	defer modelConfigMu.Unlock()
	delete(modelConfigCache, name)
}

func InvalidateAllModelConfigCache() {
	modelConfigMu.Lock()
	defer modelConfigMu.Unlock()
	modelConfigCache = make(map[string]*CacheItem)
}

func startModelConfigCleanup() {
	modelConfigCacheOnce.Do(func() {
		modelConfigCleanupCh = make(chan struct{})
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-modelConfigCleanupCh:
					return
				case <-ticker.C:
					modelConfigMu.Lock()
					now := time.Now().UnixNano()
					for key, item := range modelConfigCache {
						if now > item.Expiration {
							delete(modelConfigCache, key)
						}
					}
					modelConfigMu.Unlock()
				}
			}
		}()
	})
}
