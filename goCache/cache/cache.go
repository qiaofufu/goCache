package cache

import "time"

type Cache interface {
	Get(key string) (value Value, ok bool)             // 获取缓存
	Set(key string, value Value, expire time.Duration) // 设置缓存
	Delete(key string) (value Value, ok bool)          // 删除缓存
	RemoveOldest()                                     // 淘汰缓存
	Len() int                                          // 获取缓存记录数量
}

type entry struct {
	key    string
	value  Value
	expire int64 // UnixMilli
}

type Value interface {
	Size() int
	String() string
}

type OnEvictedFunc func(key string, value Value)
