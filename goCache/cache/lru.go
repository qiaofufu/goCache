package cache

import (
	"container/list"
	"sync"
	"time"
)

type LRU struct {
	maxBytes  int64                    // 最大缓存大小
	usedBytes int64                    // 已使用缓存大小
	ll        *list.List               // 底层链表
	mp        map[string]*list.Element // key 对 底层链表node的映射
	onEvicted OnEvictedFunc            // 淘汰缓存的回调函数
	len       int                      // entry 个数
	mu        sync.Mutex
}

func NewLRU(maxBytes int64, evictedFunc OnEvictedFunc) *LRU {
	l := &LRU{
		maxBytes:  maxBytes,
		usedBytes: 0,
		ll:        list.New(),
		mp:        make(map[string]*list.Element),
		onEvicted: evictedFunc,
	}
	return l
}

func (L *LRU) Get(key string) (value Value, ok bool) {
	L.mu.Lock()
	defer L.mu.Unlock()

	if elem, ok := L.mp[key]; ok {
		entry := elem.Value.(entry)
		if entry.expire != 0 && time.Now().UnixMilli() > entry.expire {
			L.ll.Remove(elem)
			delete(L.mp, entry.key)
			L.len--
			return nil, false
		}
		L.ll.MoveToBack(elem)
		return entry.value, true
	}
	return nil, false
}

func (L *LRU) Set(key string, value Value, expire time.Duration) {
	newEntry := entry{
		key:    key,
		value:  value,
		expire: time.Now().Add(expire).UnixMilli(),
	}

	L.mu.Lock()
	defer L.mu.Unlock()

	if elem, ok := L.mp[key]; ok {
		L.ll.Remove(elem)
		oldEntry := elem.Value.(entry)
		L.usedBytes += int64(oldEntry.value.Size()) - int64(value.Size())
		elem.Value = newEntry
	} else {
		elem = L.ll.PushBack(newEntry)
		L.mp[key] = elem
		L.usedBytes += int64(len(key)) + int64(value.Size())
		L.len++
	}

	for L.maxBytes != 0 && L.usedBytes > L.maxBytes {
		L.RemoveOldest()
	}
}

func (L *LRU) Delete(key string) (value Value, ok bool) {
	L.mu.Lock()
	defer L.mu.Unlock()

	if elem, ok := L.mp[key]; ok {
		L.ll.Remove(elem)
		delete(L.mp, key)
		return elem.Value.(entry).value, true
	}
	return nil, false
}

func (L *LRU) RemoveOldest() {
	// WARN 不可以设置锁, 外部已经设置
	f := L.ll.Front()
	e := f.Value.(entry)
	L.ll.Remove(f)
	delete(L.mp, e.key)
	L.usedBytes -= int64(len(e.key)) + int64(e.value.Size())
	L.len--
	if L.onEvicted != nil {
		L.onEvicted(e.key, e.value)
	}
}

func (L *LRU) Len() int {
	return L.len
}
