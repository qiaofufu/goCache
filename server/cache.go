package server

import (
	"goCache/server/peer"
	"time"
)

type CacheServer interface {
	Get(key string) (ByteView, error)                   // 获取缓存
	Set(key string, value []byte, expire time.Duration) // 设置缓存
	Remove(key string) error                            // 删除缓存

	removeLocally(key string) error            // 从本地删除缓存
	removeFromPeer(key string) error           // 从远程删除key
	lookupCache(key string) bool               // 查找缓存
	load(key string) (ByteView, error)         // 加载缓存
	loadLocally(key string) (ByteView, error)  // 从本地加载缓存
	loadFromPeer(key string) (ByteView, error) // 从远程加载缓存
	RegisterPeer(peer peer.Peer)               // 注册peer
}

type APIServer interface {
	StartAPIServer(addr string, cacheServer CacheServer)
}
