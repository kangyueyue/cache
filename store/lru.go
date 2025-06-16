package store

import (
	"container/list"
	"sync"
	"time"
)

// LruCache is a LRU cache.
type LruCache struct {
	mu              sync.RWMutex                  // 锁
	list            *list.List                    // 双向链表
	items           map[string]*list.Element      // 键到链表节点的映射，哈希表
	expires         map[string]time.Time          // 过期时间
	maxBytes        int64                         // 最大允许字节数
	usedBytes       int64                         // 已使用字节数
	onEvicted       func(key string, value Value) // 删除回调，某个缓存项被删除时调用
	cleanupInterval time.Duration                 // 清理间隔
	cleanupTicker   *time.Ticker                  // 清理定时器
	closeCh         chan struct{}                 // 关闭通道
}

// lruEntry is the entry of LruCache.
type lruEntry struct {
	key   string
	value Value
}

// NewLRUCache returns a new LRU cache.
func NewLRUCache(opts Options) *LruCache {
	// 设置默认时间间隔
	cleanupInterval := opts.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute // 小于0，默认一分钟
	}

	c := &LruCache{
		list:            list.New(),
		items:           make(map[string]*list.Element),
		expires:         make(map[string]time.Time),
		maxBytes:        opts.MaxBytes,
		onEvicted:       opts.OnEvicted,
		cleanupInterval: cleanupInterval,
		closeCh:         make(chan struct{}),
	}

	// 启动定时清理协程
	c.cleanupTicker = time.NewTicker(c.cleanupInterval)

	go c.cleanupLoop()
	return c
}

// Get returns the value of the key.
func (l *LruCache) Get(key string) (Value, bool) {
	l.mu.RLock()
	// 检查是否存在
	elem, ok := l.items[key]
	if !ok {
		l.mu.RUnlock()
		return nil, false
	}
	// 检查是否过期
	if expTime, hasExp := l.expires[key]; hasExp && time.Now().After(expTime) {
		l.mu.RUnlock()
		// 过期了
		go l.Delete(key) // 异步删除过期项，避免在读锁内操作
		return nil, false

	}

	// 获取值并释放锁
	entry := elem.Value.(*lruEntry)
	value := entry.value
	l.mu.RUnlock()

	// 更新LRU 位置需要写锁
	l.mu.Lock()
	// 此时再次检查，可以在获取写锁的上海被其他协程删除
	if _, ok := l.items[key]; ok {
		l.list.MoveToBack(elem) // 移动到最后
	}
	l.mu.Unlock()

	return value, true
}

// Set sets the key-value pair.
func (l *LruCache) Set(key string, value Value) error {
	return l.SetWithExpiration(key, value, 0)
}

// SetWithExpiration sets the key-value pair with expiration.
func (l *LruCache) SetWithExpiration(key string, value Value, expiration time.Duration) error {
	if value == nil {
		l.Delete(key)
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// 计算过期时间
	var expTime time.Time
	if expiration > 0 {
		expTime = time.Now().Add(expiration)
		l.expires[key] = expTime
	} else {
		delete(l.expires, key) // 删除当前key的过期时间
	}

	// 存在-更新
	if elem, ok := l.items[key]; ok {
		oldEntry := elem.Value.(*lruEntry)
		l.usedBytes += int64(value.Len() - oldEntry.value.Len())
		oldEntry.value = value
		l.list.MoveToBack(elem)
		return nil
	}

	// 不存在，添加新entry
	entry := &lruEntry{key: key, value: value}
	elem := l.list.PushBack(entry)
	l.items[key] = elem
	l.usedBytes += int64(len(key) + value.Len())

	// 检查是否需要淘汰old,超时或超内存
	l.evict()

	return nil
}

// Delete deletes the key-value pair.
func (l *LruCache) Delete(key string) bool {
	// 加写锁
	l.mu.Lock()
	defer l.mu.Unlock()
	// 检查是否存在
	if elem, ok := l.items[key]; ok {
		l.removeElement(elem)
		return true
	}
	return false
}

// Clear clears the cache.
func (l *LruCache) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 是否设置了回调函数
	if l.onEvicted != nil {
		// 遍历所有项调用回调
		for _, elem := range l.items {
			entry := elem.Value.(*lruEntry)
			l.onEvicted(entry.key, entry.value)
		}
	}

	// 重置缓存
	l.list.Init()
	l.items = make(map[string]*list.Element)
	l.expires = make(map[string]time.Time)
	l.usedBytes = 0
}

// Len returns the length of the cache.
func (l *LruCache) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.list.Len()
}

// Close closes the cache.
func (l *LruCache) Close() {
	// 关闭缓存，停止清理携程
	if l.cleanupTicker != nil {
		l.cleanupTicker.Stop()
		close(l.closeCh)
	}
}

// cleanupLoop 清理缓存
func (l *LruCache) cleanupLoop() {
	for {
		select {
		case <-l.cleanupTicker.C:
			l.mu.Lock()
			l.evict()
			l.mu.Unlock()
		case <-l.closeCh:
			return // 关闭
		}
	}
}

// evict 清理--过期或超出最大maxBytes
func (l *LruCache) evict() {
	// 先清理过期
	now := time.Now()
	for key, expTime := range l.expires {
		if now.After(expTime) {
			if elem, ok := l.items[key]; ok {
				l.removeElement(elem)
			}
		}
	}

	// 清理最久未使用的项
	for l.maxBytes > 0 && l.usedBytes > l.maxBytes && l.list.Len() > 0 {
		elem := l.list.Front() // 获取最久未使用的项（链表头部）
		if elem != nil {
			l.removeElement(elem)
		}
	}
}

// removeElement 从缓存中删除元素
func (l *LruCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*lruEntry)
	l.list.Remove(elem)        // 双向链表中删除
	delete(l.items, entry.key) // map中删除
	delete(l.expires, entry.key)
	l.usedBytes -= int64(len(entry.key) + entry.value.Len())

	// 调用回调函数
	if l.onEvicted != nil {
		l.onEvicted(entry.key, entry.value)
	}
}

// GetWithExpiration returns the value and expiration time of the key.
func (l *LruCache) GetWithExpiration(key string) (Value, time.Duration, bool) {
	// 加读锁
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 判断是否存在
	elem, ok := l.items[key]
	if !ok {
		return nil, 0, false
	}
	// 检查是否过期
	now := time.Now()
	if expTime, hasExp := l.expires[key]; hasExp {
		if now.After(expTime) {
			// 已过期
			return nil, 0, false
		}

		// 计算剩余时间
		ttl := expTime.Sub(now)
		l.list.MoveToBack(elem) // 使用过，调用到最后
		return elem.Value.(*lruEntry).value, ttl, true
	}

	// 无过期时间
	l.list.MoveToBack(elem)
	return elem.Value.(*lruEntry).value, 0, true
}

// GetExpiration returns the expiration time of
func (l *LruCache) GetExpiration(key string) (time.Time, bool) {
	// 加读锁
	l.mu.RLock()
	defer l.mu.RUnlock()

	expTime, ok := l.expires[key]
	return expTime, ok
}

// UpdateExpiration updates the expiration time of the key.
func (l *LruCache) UpdateExpiration(key string, expiration time.Duration) bool {
	// 加读锁
	l.mu.RLock()
	defer l.mu.RUnlock()

	if _, ok := l.items[key]; !ok {
		return false
	}

	if expiration > 0 {
		l.expires[key] = time.Now().Add(expiration)
	} else {
		delete(l.expires, key)
	}

	return true
}

// UsedBytes returns the used bytes of the cache.
func (l *LruCache) UsedBytes() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.usedBytes
}

// MaxBytes returns the max bytes of the cache.
func (l *LruCache) MaxBytes() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.maxBytes
}

// SetMaxBytes sets the max bytes of the cache.
func (l *LruCache) SetMaxBytes(maxBytes int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxBytes = maxBytes

	if maxBytes > 0 {
		// 触发淘汰
		l.evict()
	}
}
