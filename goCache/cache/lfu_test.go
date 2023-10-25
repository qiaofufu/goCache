package cache

import (
	"testing"
	"time"
)

func TestLFUCache(t *testing.T) {
	lfu := NewLFU(1<<32, nil) // 替换为你的 LFU 缓存初始化方式，传入合适的最大缓存大小

	// 添加缓存条目
	lfu.Set("key1", NewValue("value1"), time.Second)
	lfu.Set("key2", NewValue("value2"), time.Second)

	// 获取缓存条目
	value, ok := lfu.Get("key1")
	if !ok || value.String() != "value1" {
		t.Errorf("Get(\"key1\") = %v, want %v", value, "value1")
	}

	// 测试删除缓存
	lfu.Delete("key1")
	_, ok = lfu.Get("key1")
	if ok {
		t.Errorf("Key \"key1\" should have been deleted")
	}

	// 测试淘汰策略
	lfu.Set("key3", NewValue("value3"), time.Second)
	lfu.Set("key4", NewValue("value4"), time.Second)
	lfu.Set("key5", NewValue("value5"), time.Second)
	lfu.Set("key6", NewValue("value6"), time.Second)

	if lfu.Len() > 4 {
		t.Errorf("LFU cache should have been limited to 4 items")
	}
}

func TestLFUCacheEvictionCallback(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	values := []string{"10", "20", "30"}
	totalSize := 0
	// Create an LRU cache with a capacity of 50
	for i := range keys {
		totalSize += len(keys[i]) + len(values[i])
	}
	lru := NewLFU(int64(totalSize-5), func(key string, value Value) {
		t.Logf("evicted key:%s value:%s", key, value)
	})

	// Set values that will exceed the cache capacity
	lru.Set("key1", NewValue("10"), time.Second*2)
	lru.Set("key2", NewValue("20"), time.Second*2)
	lru.Set("key3", NewValue("30"), time.Second*2)

	// Check if the oldest value was evicted
	val, ok := lru.Get("key1")
	if ok || val != nil {
		t.Errorf("Expected key1 to be evicted, got %v", val)
	}
}
