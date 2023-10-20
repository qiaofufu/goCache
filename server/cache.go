package server

import (
	"fmt"
	"goCache/server/option"
	"goCache/server/singleflight"
	"sync"
	"time"
)

type Cache struct {
	name string
	option.CacheOption
	peer   Peer
	loader singleflight.Flight
}

var (
	caches = make(map[string]*Cache)
	mu     sync.RWMutex
)

func GetCache(namespace string) *Cache {
	mu.RLock()
	defer mu.RUnlock()
	return caches[namespace]
}

func NewCache(name string, options ...option.CacheOptionFunc) *Cache {
	mu.Lock()
	defer mu.Unlock()

	cache := &Cache{
		name:        name,
		CacheOption: option.DefaultCacheOption(),
	}
	for _, op := range options {
		op(&cache.CacheOption)
	}
	caches[name] = cache
	return cache
}

func (c *Cache) Get(key string) (ByteView, error) {
	if exist := c.lookupCache(key); exist {
		if value, ok := c.Strategy.Get(key); ok {
			return value.(ByteView), nil
		}
		return ByteView{}, fmt.Errorf("local cache not get %s", key)
	}
	return c.load(key)
}

func (c *Cache) Set(key string, value []byte, expire time.Duration) {
	c.Strategy.Set(key, ByteView{value}, expire)
}

func (c *Cache) Remove(key string) error {
	if _, ok := c.Strategy.Delete(key); !ok {
		return fmt.Errorf("failed to delete cache, key: %v", key)
	}
	return nil
}

func (c *Cache) removeLocally(key string) error {
	//TODO implement me
	panic("implement me")
}

func (c *Cache) removeFromPeer(key string) error {
	//TODO implement me
	panic("implement me")
}

func (c *Cache) lookupCache(key string) bool {
	//TODO implement me
	panic("implement me")
}

func (c *Cache) load(key string) (ByteView, error) {
	c.loader.Do(key, func() (interface{}, error) {
		peer, err := c.peer.PickerPeer(key)
		if err != nil {
			return ByteView{}, err
		}
		fmt.Printf("select perr [%v]\n", peer.Addr())
		return peer.Get(c.name, key)
	})
}

func (c *Cache) loadLocally(key string) (ByteView, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Cache) loadFromPeer(key string) (ByteView, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Cache) RegisterPeer(peer Peer) {
	c.peer = peer
}
