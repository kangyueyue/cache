package store

import "time"

// Value is the interface that wraps the Len method.
type Value interface {
	Len() int
}

// Store is the interface that wraps the basic Get and Set methods.
type Store interface {
	Get(key string) (Value, bool)
	Set(key string, value Value) error
	SetWithExpiration(key string, value Value, expiration time.Duration) error
	Delete(key string) bool
	Clear()
	Len() int
	Close()
}

// CacheType is the type of cache
type CacheType string

const (
	LRU CacheType = "lru"
	LFU CacheType = "lfu"
)

// Options is the options for cache
type Options struct {
	MaxBytes        int64                         // 最大字节
	CleanupInterval time.Duration                 // 清理间隔
	OnEvicted       func(key string, value Value) // 删除回调
}

// DefaultOptions returns the default options
func DefaultOptions() Options {
	return Options{
		MaxBytes:        1024 * 8,    // 8KB
		CleanupInterval: time.Minute, // 1min
		OnEvicted:       nil,         // nil
	}
}

// NewOptions returns the default options
func NewOptions() Options {
	return DefaultOptions()
}

// NewStore returns a new Store
func NewStore(cacheType CacheType, opts Options) Store {
	switch cacheType {
	case LRU:
		return NewLRUCache(opts)
	case LFU:
		return NewLFUCache(opts)
	default:
		return NewLFUCache(opts)
	}
}
