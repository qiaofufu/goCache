package storage

import (
	"testing"
	"time"
)

type NewValue string

func (n NewValue) Size() int {
	return len(n)
}

func (n NewValue) String() string {
	return string(n)
}

func TestLRU_Expire(t *testing.T) {
	// Create an LRU cache with a capacity of 100
	lru := NewLRU(100, nil)

	// Set a value with a short expiration time
	lru.Set("key1", NewValue("10"), time.Millisecond)

	// Wait for the value to expire
	time.Sleep(time.Millisecond * 10)

	// Attempt to get the expired value
	val, ok := lru.Get("key1")
	if ok || val != nil {
		t.Errorf("Expected key1 to be expired, got %v", val)
	}
}

func TestLRU_Eviction(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	values := []string{"10", "20", "30"}
	totalSize := 0
	// Create an LRU cache with a capacity of 50
	for i := range keys {
		totalSize += len(keys[i]) + len(values[i])
	}
	lru := NewLRU(int64(totalSize-10), nil)

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

func TestLRU_Delete(t *testing.T) {
	// Create an LRU cache with a capacity of 100
	lru := NewLRU(100, nil)

	// Set a value
	lru.Set("key1", NewValue("10"), time.Second*20)

	// Delete the value
	val, ok := lru.Delete("key1")
	if !ok || val.String() != "10" {
		t.Errorf("Expected key1=10, got %v", val)
	}

	// Attempt to get the deleted value
	val, ok = lru.Get("key1")
	if ok || val != nil {
		t.Errorf("Expected key1 to be deleted, got %v", val)
	}
}
