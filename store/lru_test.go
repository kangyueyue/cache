package store

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 测试value
type testValue string

func (v testValue) Len() int {
	return len(v)
}

// 测试LRU缓存
func TestLRUCacheSet(t *testing.T) {
	lruCache := NewLRUCache(Options{
		MaxBytes: 1024, // 1KB
		OnEvicted: func(key string, value Value) {
			t.Logf("Evicted: %s -> %s", key, value)
		},
		CleanupInterval: 3 * time.Second,
	})
	err := lruCache.Set("1", testValue("1"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestLRUCacheGet(t *testing.T) {
	lruCache := NewLRUCache(Options{
		MaxBytes: 1024, // 1KB
		OnEvicted: func(key string, value Value) {
			t.Logf("Evicted: %s -> %s", key, value)
		},
		CleanupInterval: 3 * time.Second,
	})
	err := lruCache.Set("1", testValue("1"))
	if err != nil {
		t.Fatal(err)
	}
	val, ok := lruCache.Get("1")
	if !ok {
		t.Fatalf("Get failed")
	}
	assert.Equal(t, val, testValue("1"))
}

func TestLRUCacheDelete(t *testing.T) {
	lruCache := NewLRUCache(Options{
		MaxBytes: 10, // 1KB
		OnEvicted: func(key string, value Value) {
			t.Logf("Evicted: %s -> %s", key, value)
		},
		CleanupInterval: 3 * time.Second,
	})
	// 加入五个元素
	for i := range 5 {
		err := lruCache.Set(strconv.Itoa(i), testValue(strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
	}
	// 1-4使用2次
	for j := range 5 {
		_, ok := lruCache.Get(strconv.Itoa(4 - j))
		if !ok {
			t.Fatalf("Get failed: %d", 4-j)
		}
	}

	// 加入5，应该淘汰4
	err := lruCache.Set("5", testValue("5"))
	if err != nil {
		t.Fatal(err)
	}
	_, ok := lruCache.Get("4")
	assert.False(t, ok)
}
