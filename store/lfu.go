package store

import (
	"container/list"
	"sync"
	"time"
)

// LfuCache is a LFU cache.
type LfuCache struct {
	mu              sync.RWMutex                  // 读写锁
	kItems          map[string]*list.Element      // k map key 到链表节点的映射，哈希表
	fItems          map[int64]*list.List          // f map 次数频率为key 到链表的映射，哈希表
	minFre          int64                         // 最小频率
	expires         map[string]time.Time          // 过期时间
	maxBytes        int64                         // 最大允许字节数
	usedBytes       int64                         // 已使用字节数
	onEvicted       func(key string, value Value) // 删除回调，某个缓存项被删除时调用
	cleanupInterval time.Duration                 // 清理间隔
	cleanupTicker   *time.Ticker                  // 清理定时器
	closeCh         chan struct{}                 // 关闭通道
}

// lfuEntry is the entry of LfuCache.
type lfuEntry struct {
	key   string
	value Value
	freq  int64
}

// NewLFUCache returns a new LRU cache.
func NewLFUCache(opts Options) *LfuCache {
	// 设置默认时间间隔
	cleanupInterval := opts.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute // 小于0，默认一分钟
	}

	l := &LfuCache{
		kItems:          make(map[string]*list.Element),
		fItems:          make(map[int64]*list.List),
		minFre:          1, // 默认为1
		expires:         make(map[string]time.Time),
		maxBytes:        opts.MaxBytes,
		onEvicted:       opts.OnEvicted,
		cleanupInterval: cleanupInterval,
		closeCh:         make(chan struct{}),
	}

	l.fItems[l.minFre] = list.New() // 初始化最小频率链表

	// 启动定时清理协程
	l.cleanupTicker = time.NewTicker(l.cleanupInterval)

	go l.cleanupLoop()

	return l
}

// Get returns the value of the key.
func (l *LfuCache) Get(key string) (Value, bool) {
	l.mu.RLock()
	// 获取key对应的元素
	elem, ok := l.kItems[key]
	if !ok {
		l.mu.RUnlock()
		return nil, false
	}
	// 检查是否过期
	if expTime, hasExp := l.expires[key]; hasExp && time.Now().After(expTime) {
		l.mu.RUnlock()
		// 删除过期项
		go l.Delete(key)
		return nil, false
	}

	// 获取值并释放锁
	entry := elem.Value.(*lfuEntry)
	value := entry.value
	l.mu.RUnlock()

	// 更新LFU 位置需要写锁
	l.mu.Lock()
	// 此时再次检查，可以在获取写锁的上海被其他协程删除
	if _, ok := l.kItems[key]; ok {
		l.handleEntry(elem) // 更新位置
	}
	l.mu.Unlock()

	return value, true
}

// Set sets the key-value pair.
func (l *LfuCache) Set(key string, value Value) error {
	return l.SetWithExpiration(key, value, 0)
}

// SetWithExpiration sets the key-value pair with expiration.
func (l *LfuCache) SetWithExpiration(key string, value Value, expiration time.Duration) error {
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
		l.expires[key] = expTime //设置过期时间
	} else {
		delete(l.expires, key) // 删除当前key的过期时间
	}

	// 存在-更新
	if elem, ok := l.kItems[key]; ok {
		oldEntry := elem.Value.(*lfuEntry)
		l.usedBytes += int64(value.Len() - oldEntry.value.Len())
		oldEntry.value = value
		l.handleEntry(elem)
		return nil
	}

	// 不存在-新增
	if l.usedBytes == l.maxBytes {
		// 删除最少使用的元素
		fl := l.fItems[l.minFre]
		l.removeElement(fl.Front())
	}
	entry := &lfuEntry{
		key:   key,
		value: value,
		freq:  1,
	}
	elem := l.fItems[entry.freq].PushBack(entry)
	l.kItems[key] = elem
	l.minFre = 1 // 更新最小频率
	l.usedBytes += int64(len(key) + value.Len())

	return nil
}

// Delete deletes the key-value pair.
func (l *LfuCache) Delete(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if elem, ok := l.kItems[key]; ok {
		l.removeElement(elem)
		return true
	}
	return false
}

// Clear clears the cache.
func (l *LfuCache) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.onEvicted != nil {
		// 遍历所有项调用回调
		for _, elem := range l.kItems {
			entry := elem.Value.(*lfuEntry)
			l.onEvicted(entry.key, entry.value)
		}
	}

	l.kItems = make(map[string]*list.Element)
	l.fItems = make(map[int64]*list.List)
	l.expires = make(map[string]time.Time)
	l.usedBytes = 0
	l.minFre = 1
}

// Len returns the number of items in the cache.
func (l *LfuCache) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.kItems)
}

// Close closes the cache.
func (l *LfuCache) Close() {
	if l.cleanupTicker != nil {
		l.cleanupTicker.Stop()
		close(l.closeCh) // 关闭channel
	}
}

// cleanupLoop 清理过期数据
func (l *LfuCache) cleanupLoop() {
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

// evict 清理过期或超过最大内存数据
func (l *LfuCache) evict() {
	now := time.Now()
	for key, expTime := range l.expires {
		if now.After(expTime) {
			if elem, ok := l.kItems[key]; ok {
				l.removeElement(elem)
			}
		}
	}

	// 超过最大内存清理最久未使用的项
	for l.maxBytes > 0 && l.usedBytes > l.maxBytes && len(l.kItems) > 0 {
		fl := l.fItems[l.minFre]
		if fl != nil && fl.Len() > 0 {
			l.removeElement(fl.Front())
		} else {
			// 重新定位最小频率
			min := int64(0)
			for freq, lst := range l.fItems {
				if lst != nil && lst.Len() > 0 {
					if min == 0 || freq < min {
						min = freq
					}
				}
			}
			l.minFre = min
			// 如果找到有效链表则继续淘汰
			if min > 0 {
				if fl := l.fItems[min]; fl != nil && fl.Len() > 0 {
					l.removeElement(fl.Front())
				}
			}
		}
	}
}

// handleEntry 执行，处理频率等
func (l *LfuCache) handleEntry(elem *list.Element) {
	// 从原频率中删除
	kv := elem.Value.(*lfuEntry)
	oldList := l.fItems[kv.freq]
	oldList.Remove(elem)

	// 更新minFre
	if oldList.Len() == 0 && l.minFre == kv.freq {
		l.minFre++
	}

	// 放入新的频率链表
	kv.freq++
	if _, ok := l.fItems[kv.freq]; !ok {
		// 不存在-创建
		l.fItems[kv.freq] = list.New()
	}
	// 放入新的频率链表
	newList := l.fItems[kv.freq]
	front := newList.PushFront(kv)
	l.kItems[kv.key] = front // 更新k map
}

// removeElement 删除元素
func (l *LfuCache) removeElement(elem *list.Element) {
	kv := elem.Value.(*lfuEntry)
	l.fItems[kv.freq].Remove(elem) // 从频率List删除
	delete(l.kItems, kv.key)       // 从k map删除
	delete(l.expires, kv.key)      // 从expires map删除
	l.usedBytes -= int64(len(kv.key)) + int64(kv.value.Len())

	// 调用回调函数
	if l.onEvicted != nil {
		l.onEvicted(kv.key, kv.value)
	}
}

// GetWithExpiration 获取并更新过期时间
func (l *LfuCache) GetWithExpiration(key string) (Value, time.Duration, bool) {
	l.mu.RLock()
	// 获取key对应的元素
	elem, ok := l.kItems[key]
	if !ok {
		l.mu.RUnlock()
		return nil, 0, false
	}
	// 检查是否过期
	if expTime, hasExp := l.expires[key]; hasExp && time.Now().After(expTime) {
		l.mu.RUnlock()
		return nil, 0, false
	}

	// 获取值并释放锁
	entry := elem.Value.(*lfuEntry)
	value := entry.value
	ttl := l.expires[key].Sub(time.Now()) // 计算剩余时间
	l.mu.RUnlock()

	// 更新LFU 位置需要写锁
	l.mu.Lock()
	// 此时再次检查，可以在获取写锁的上海被其他协程删除
	if _, ok := l.kItems[key]; ok {
		l.handleEntry(elem) // 更新位置
	}
	l.mu.Unlock()

	return value, ttl, true
}

// GetExpiration returns the expiration time of
func (l *LfuCache) GetExpiration(key string) (time.Time, bool) {
	// 加读锁
	l.mu.RLock()
	defer l.mu.RUnlock()

	expTime, ok := l.expires[key]
	return expTime, ok
}

// UpdateExpiration updates the expiration time of the key.
func (l *LfuCache) UpdateExpiration(key string, expiration time.Duration) bool {
	// 加读锁
	l.mu.RLock()
	defer l.mu.RUnlock()

	if _, ok := l.kItems[key]; !ok {
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
func (l *LfuCache) UsedBytes() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.usedBytes
}

// MaxBytes returns the max bytes of the cache.
func (l *LfuCache) MaxBytes() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.maxBytes
}

// SetMaxBytes sets the max bytes of the cache.
func (l *LfuCache) SetMaxBytes(maxBytes int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxBytes = maxBytes

	if maxBytes > 0 {
		// 触发淘汰
		l.evict()
	}
}
