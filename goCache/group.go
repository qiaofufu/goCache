package goCache

import (
	"fmt"
	"goCache/goCache/singleflight"
	"log"
	"math/rand"
	"sync"
	"time"
)

const (
	defaultExpire = time.Second * 30
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
	CacheOption
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

func NewGroup(name string, getter Getter, options ...CacheOptionFunc) *Group {
	mu.Lock()
	defer mu.Unlock()

	cache := &Group{
		name:        name,
		getter:      getter,
		CacheOption: DefaultCacheOption(),
	}
	for _, op := range options {
		op(&cache.CacheOption)
	}
	groups[name] = cache
	return cache
}

func (c *Group) Get(key string) (ByteView, error) {
	if v, exist := c.lookupCache(key); exist {
		log.Println("lookup cache success, key:", key)
		return v, nil
	}
	log.Println("lookup cache fail, key:", key)
	return c.load(key)
}

func (c *Group) Set(key string, value []byte, expire time.Duration) {
	c.mainCache.Set(key, ByteView{value}, expire)
}

func (c *Group) Remove(key string) error {
	if _, exist := c.lookupCache(key); exist {
		return c.removeLocally(key)
	}
	return c.removeFromPeer(key)
}

func (c *Group) removeLocally(key string) error {
	_, ok := c.mainCache.Delete(key)
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
	v, ok := c.mainCache.Get(key)
	if ok {
		return v.(ByteView), ok
	}
	v, ok = c.hotCache.Get(key)
	if !ok {
		return ByteView{}, false
	}
	return v.(ByteView), ok
}

func (c *Group) load(key string) (ByteView, error) {
	value, err := c.loader.Do(key, func() (interface{}, error) {
		peer, ok := c.peer.PickPeer(key)
		if ok {
			return c.loadFromPeer(key, peer)
		}
		return c.loadLocally(key)
	})
	c.Set(key, value.(ByteView).Slice(), defaultExpire)
	return value.(ByteView), err
}

func (c *Group) loadLocally(key string) (ByteView, error) {
	v, err := c.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: v}, nil
}

func (c *Group) loadFromPeer(key string, peer PeerGetter) (ByteView, error) {
	data, err := peer.Get(c.name, key)
	if err != nil {
		return ByteView{}, err
	}
	if rand.Intn(10) == 0 {
		c.hotCache.Set(key, ByteView{b: data}, defaultExpire)
	}
	return ByteView{b: data}, nil
}

func (c *Group) RegisterPeer(peer Peer) {
	c.peer = peer
}
