package cache

import (
	"container/list"
	"sync"
	"time"
)

type LFU struct {
	freq       map[int64]*list.List
	mp         map[string]*list.Element
	mp2        map[string]int64
	curMinFreq int64
	maxBytes   int64         // 最大缓存大小
	usedBytes  int64         // 已使用缓存大小
	onEvicted  OnEvictedFunc // 淘汰缓存的回调函数
	len        int           // entry 个数
	mu         sync.Mutex
}

func NewLFU(maxBytes int64, evictedFunc OnEvictedFunc) *LFU {
	l := &LFU{
		maxBytes:   maxBytes,
		usedBytes:  0,
		freq:       make(map[int64]*list.List),
		curMinFreq: 1,
		mp:         make(map[string]*list.Element),
		mp2:        make(map[string]int64),
		onEvicted:  evictedFunc,
	}
	return l
}

type LFUEntry struct {
	entry
	freq int64
}

func (L *LFU) Get(key string) (value Value, ok bool) {
	L.mu.Lock()
	defer L.mu.Unlock()
	if elem, ok := L.mp[key]; ok {
		entry := elem.Value.(LFUEntry)
		L.incr(entry)
		return entry.value, true
	}
	return nil, false
}

func (L *LFU) Set(key string, value Value, expire time.Duration) {
	L.mu.Lock()
	defer L.mu.Unlock()
	if elem, ok := L.mp[key]; ok {
		entry := elem.Value.(LFUEntry)
		oldSize := entry.value.Size()
		entry.value = value
		entry.expire = int64(expire)
		L.incr(entry)
		L.usedBytes += int64(value.Size()) - int64(oldSize)
	} else {
		entry := LFUEntry{
			entry: entry{
				key:    key,
				value:  value,
				expire: int64(expire),
			},
			freq: 1,
		}
		L.add(entry)
	}

	for L.maxBytes != 0 && L.usedBytes > L.maxBytes {
		L.RemoveOldest()
	}
}

func (L *LFU) incr(entry LFUEntry) {
	elem := L.mp[entry.key]
	L.freq[entry.freq].Remove(elem)
	if L.freq[entry.freq].Len() == 0 {
		delete(L.freq, entry.freq)
		if entry.freq == L.curMinFreq {
			L.curMinFreq++
		}
	}
	entry.freq++
	L.add(entry)
}

func (L *LFU) add(entry LFUEntry) {
	if _, ok := L.freq[entry.freq]; !ok {
		L.freq[entry.freq] = list.New()
	}
	L.mp[entry.key] = L.freq[entry.freq].PushBack(entry)
	L.mp2[entry.key] = entry.freq
	L.usedBytes += int64(len(entry.key)) + int64(entry.value.Size())
}

func (L *LFU) Delete(key string) (value Value, ok bool) {
	L.mu.Lock()
	defer L.mu.Unlock()
	if freq, ok := L.mp2[key]; ok {
		l := L.freq[freq]
		elem := L.mp[key]
		l.Remove(elem)
		delete(L.mp, key)
		delete(L.mp2, key)
	}
	return nil, true
}

func (L *LFU) RemoveOldest() {
	l := L.freq[L.curMinFreq]
	elem := l.Front()
	entry := elem.Value.(LFUEntry)
	l.Remove(elem)
	delete(L.mp, entry.key)
	delete(L.mp2, entry.key)
	L.usedBytes -= int64(len(entry.key)) + int64(entry.value.Size())
	if L.onEvicted != nil {
		L.onEvicted(entry.key, entry.value)
	}
	for tmpl, ok := L.freq[L.curMinFreq]; ; tmpl, ok = L.freq[L.curMinFreq] {
		if !ok {
			L.curMinFreq++
			continue
		}
		if tmpl.Len() == 0 {
			delete(L.freq, L.curMinFreq)
			L.curMinFreq++
			continue
		}
		break
	}
}

func (L *LFU) Len() int {
	return L.len
}
