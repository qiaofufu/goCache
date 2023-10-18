package storage

import "time"

type Storage interface {
	Get(key string) (value Value, ok bool)             // 获取缓存
	Set(key string, value Value, expire time.Duration) // 设置缓存
	Delete(key string) (value Value, ok bool)          // 删除缓存
	SetOnEvicted(onEvicted OnEvictedFunc)              // 设置缓存淘汰的回调函数
	Len() int                                          // 获取缓存记录数量
}

type Value interface {
	Size() int
}

type OnEvictedFunc func(key string, value Value)
