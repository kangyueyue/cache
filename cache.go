package kamacache

import (
	"context"
	"github.com/zuozikang/cache/store"
	"github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
	"time"
)

// Cache 是对底层缓存存储的封装
type Cache struct {
	mu          sync.RWMutex
	store       store.Store
	opts        CacheOptions
	hits        int64 // 命中次数
	misses      int64 // 未命中次数
	initialized int32 // 原子变量，标记缓存是否已初始化
	closed      int32 // 原子变量，标记缓存是否已关闭
}

// CacheOptions opts
type CacheOptions struct {
	CacheType       store.CacheType                     // 类型 lru lfu
	MaxBytes        int64                               // 最大字节数
	CleanupInterval time.Duration                       // 清理间隔
	OnEvicted       func(key string, value store.Value) // 驱逐回调
}

// DefaultCacheOptions 返回默认值
func DefaultCacheOptions() CacheOptions {
	return CacheOptions{
		CacheType:       store.LFU,
		MaxBytes:        8 * 1024 * 1024, // 8MB
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}
}

// NewCache 返回最新的缓存示例
func NewCache(opts CacheOptions) *Cache {
	return &Cache{
		opts: opts,
	}
}

// 确保已经初始化，避免不必要的锁竞争
func (c *Cache) ensureInitialized() {
	if atomic.LoadInt32(&c.initialized) == 1 {
		return
	}

	// 双重检查锁定模式
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized == 0 {
		// 创建store opts
		storeOpts := store.Options{
			MaxBytes:        c.opts.MaxBytes,
			CleanupInterval: c.opts.CleanupInterval,
			OnEvicted:       c.opts.OnEvicted,
		}

		c.store = store.NewStore(c.opts.CacheType, storeOpts)

		// 标记为1,已初始化
		atomic.StoreInt32(&c.initialized, 1)

		logrus.Infof("Cache initialized with type %s, max bytes: %d", c.opts.CacheType, c.opts.MaxBytes)
	}
}

// Add 加入一个键值对到缓存中
func (c *Cache) Add(key string, value ByteView) {
	if atomic.LoadInt32(&c.closed) == 1 {
		logrus.Warnf("Attempted to add to a closed cache: %s", key)
		return
	}
	c.ensureInitialized()

	if err := c.store.Set(key, value); err != nil {
		logrus.Warnf("Fail to add key %s to cache:%v", key, err)
	}
}

// Get 从缓存中获取key value
func (c *Cache) Get(ctx context.Context, key string) (ByteView, bool) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return ByteView{}, false
	}
	// 未初始化
	if atomic.LoadInt32(&c.initialized) == 0 {
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}

	// add read lock
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 从底层存储获取
	val, found := c.store.Get(key)
	if !found {
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}
	// 更新命中率
	atomic.AddInt64(&c.hits, 1)

	// 转换返回
	if bv, ok := val.(ByteView); ok {
		return bv, true
	}

	// 尝试断言失败
	logrus.Warnf("Type assertion failed for key %s, expected ByteView", key)
	atomic.AddInt64(&c.misses, 1)
	return ByteView{}, false
}

// AddWithExpiration 向缓存中添加一个带过期时间的 key-value 对
func (c *Cache) AddWithExpiration(key string, value ByteView, expirationTime time.Time) {
	if atomic.LoadInt32(&c.closed) == 1 {
		logrus.Warnf("Attempted to add to a closed cache: %s", key)
		return
	}

	c.ensureInitialized()

	// 计算过期时间
	expiration := time.Until(expirationTime)
	if expiration <= 0 {
		logrus.Debugf("Key %s already expired, not adding to cache", key)
		return
	}
	// 设置到底层
	if err := c.store.SetWithExpiration(key, value, expiration); err != nil {
		logrus.Warnf("Failed to add key %s to cache with expiration: %v", key)
	}
}

// Delete 从缓存中删除一个 key
func (c *Cache) Delete(key string) bool {
	// 未初始化或已关闭
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.store.Delete(key)
}

// Clear 清空缓存
func (c *Cache) Clear() {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	c.store.Clear()

	// 重置
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}

// Len 返回缓存的当前存储项数量
func (c *Cache) Len() int {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.store.Len()
}

// Close 关闭缓存，释放资源
func (c *Cache) Close() {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// 关闭底层存储
	if c.store != nil {
		if closer, ok := c.store.(interface{ Close() }); ok {
			closer.Close()
		}
		c.store = nil
	}
	// 充值缓存状态
	atomic.StoreInt32(&c.initialized, 0)

	logrus.Debugf("Cache closed, hits: %d, misses: %d", atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses))
}

// Stats 返回缓存统计信息
func (c *Cache) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"initialized": atomic.LoadInt32(&c.initialized) == 1,
		"closed":      atomic.LoadInt32(&c.closed) == 1,
		"hits":        atomic.LoadInt64(&c.hits),
		"misses":      atomic.LoadInt64(&c.misses),
	}

	// 已初始化
	if atomic.LoadInt32(&c.initialized) == 1 {
		stats["size"] = c.Len()

		// 计算命中率
		totalRequests := stats["hits"].(int64) + stats["misses"].(int64)
		if totalRequests > 0 {
			stats["hit_rate"] = float64(stats["hits"].(int64)) / float64(totalRequests)
		} else {
			stats["hit_rate"] = 0.0
		}
	}

	return stats
}
