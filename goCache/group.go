package goCache

import (
	"fmt"
	"goCache/goCache/option"
	"goCache/goCache/singleflight"
	"log"
	"sync"
	"time"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

type Group struct {
	name   string
	getter Getter
	option.CacheOption
	peer   Peer
	loader singleflight.Flight
}

var (
	groups = make(map[string]*Group)
	mu     sync.RWMutex
)

func GetGroup(group string) (*Group, bool) {
	mu.RLock()
	defer mu.RUnlock()

	if g, ok := groups[group]; ok {
		return g, true
	}
	return nil, false
}

func NewGroup(name string, getter Getter, options ...option.CacheOptionFunc) *Group {
	mu.Lock()
	defer mu.Unlock()

	cache := &Group{
		name:        name,
		getter:      getter,
		CacheOption: option.DefaultCacheOption(),
	}
	for _, op := range options {
		op(&cache.CacheOption)
	}
	groups[name] = cache
	return cache
}

func (c *Group) Get(key string) (ByteView, error) {
	if v, exist := c.lookupCache(key); exist {
		return v, nil
	}
	return c.load(key)
}

func (c *Group) Set(key string, value []byte, expire time.Duration) {
	c.Strategy.Set(key, ByteView{value}, expire)
}

func (c *Group) Remove(key string) error {
	if _, exist := c.lookupCache(key); exist {
		return c.removeLocally(key)
	}
	return c.removeFromPeer(key)
}

func (c *Group) removeLocally(key string) error {
	_, ok := c.Strategy.Delete(key)
	if !ok {
		return fmt.Errorf("failed to remove, key: %s", key)
	}
	return nil
}

func (c *Group) removeFromPeer(key string) error {
	peer, ok := c.peer.PickPeer(key)
	if !ok {
		return fmt.Errorf("picker not have")
	}
	return peer.Remove(c.name, key)
}

func (c *Group) lookupCache(key string) (ByteView, bool) {
	v, ok := c.Strategy.Get(key)
	if !ok {
		return ByteView{}, false
	}
	return v.(ByteView), ok
}

func (c *Group) load(key string) (ByteView, error) {
	log.Println("load cache, key:", key)
	value, err := c.loader.Do(key, func() (interface{}, error) {
		peer, ok := c.peer.PickPeer(key)
		if ok {
			return c.loadFromPeer(key, peer)
		}
		return c.loadLocally(key)
	})

	return value.(ByteView), err
}

func (c *Group) loadLocally(key string) (ByteView, error) {
	log.Println("load cache in local, key:", key)
	v, err := c.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: v}, nil
}

func (c *Group) loadFromPeer(key string, peer PeerGetter) (ByteView, error) {
	log.Printf("load cache in remote[%s], key:%s\n", peer.Addr(), key)
	data, err := peer.Get(c.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: data}, nil
}

func (c *Group) RegisterPeer(peer Peer) {
	c.peer = peer
}
