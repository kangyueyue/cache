package store

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

// 测试LRU缓存
func TestLFUCacheSet(t *testing.T) {
	lfuCache := NewLFUCache(Options{
		MaxBytes: 1024, // 1KB
		OnEvicted: func(key string, value Value) {
			t.Logf("Evicted: %s -> %s", key, value)
		},
		CleanupInterval: 3 * time.Second,
	})
	err := lfuCache.Set("1", testValue("1"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestLFUCacheGet(t *testing.T) {
	lfuCache := NewLFUCache(Options{
		MaxBytes: 1024, // 1KB
		OnEvicted: func(key string, value Value) {
			t.Logf("Evicted: %s -> %s", key, value)
		},
		CleanupInterval: 3 * time.Second,
	})
	err := lfuCache.Set("1", testValue("1"))
	if err != nil {
		t.Fatal(err)
	}
	val, ok := lfuCache.Get("1")
	if !ok {
		t.Fatalf("Get failed")
	}
	assert.Equal(t, val, testValue("1"))
}

func TestLFUCacheDelete(t *testing.T) {
	lfuCache := NewLFUCache(Options{
		MaxBytes: 12,
		OnEvicted: func(key string, value Value) {
			t.Logf("Evicted: %s -> %s", key, value)
		},
		CleanupInterval: 3 * time.Second,
	})
	// 加入6个元素
	for i := range 6 {
		err := lfuCache.Set(strconv.Itoa(i), testValue(strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := range 6 {
		for j := range i {
			t.Logf("Get: %s, %v", strconv.Itoa(i), j+1)
			val, ok := lfuCache.Get(strconv.Itoa(i))
			assert.True(t, ok)
			assert.Equal(t, val, testValue(strconv.Itoa(i)))
		}
	}
	// 加入6，应该淘汰0
	err := lfuCache.Set("6", testValue("6"))
	if err != nil {
		t.Fatal(err)
	}
	_, ok := lfuCache.Get("0")
	assert.False(t, ok)
}
