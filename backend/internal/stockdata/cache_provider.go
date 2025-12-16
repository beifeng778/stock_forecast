package stockdata

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type CacheProvider interface {
	Get(key string, dest any) error
	Set(key string, value any, expiration time.Duration) error
}

type inMemoryCacheItem struct {
	data      []byte
	expiresAt time.Time
}

type InMemoryCacheProvider struct {
	mu    sync.RWMutex
	items map[string]inMemoryCacheItem
}

func NewInMemoryCacheProvider() *InMemoryCacheProvider {
	return &InMemoryCacheProvider{items: map[string]inMemoryCacheItem{}}
}

func (p *InMemoryCacheProvider) Get(key string, dest any) error {
	if p == nil {
		return fmt.Errorf("cache provider is nil")
	}
	p.mu.RLock()
	item, ok := p.items[key]
	p.mu.RUnlock()
	if !ok {
		return fmt.Errorf("cache miss")
	}
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		p.mu.Lock()
		delete(p.items, key)
		p.mu.Unlock()
		return fmt.Errorf("cache expired")
	}
	if len(item.data) == 0 {
		return fmt.Errorf("cache empty")
	}
	return json.Unmarshal(item.data, dest)
}

func (p *InMemoryCacheProvider) Set(key string, value any, expiration time.Duration) error {
	if p == nil {
		return fmt.Errorf("cache provider is nil")
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var expiresAt time.Time
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}
	p.mu.Lock()
	p.items[key] = inMemoryCacheItem{data: b, expiresAt: expiresAt}
	p.mu.Unlock()
	return nil
}

var cacheProvider CacheProvider = NewInMemoryCacheProvider()

func SetCacheProvider(p CacheProvider) {
	if p == nil {
		cacheProvider = NewInMemoryCacheProvider()
		return
	}
	cacheProvider = p
}

func getCacheProvider() CacheProvider {
	if cacheProvider == nil {
		cacheProvider = NewInMemoryCacheProvider()
	}
	return cacheProvider
}
