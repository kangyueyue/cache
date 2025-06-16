package consistenthash

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ConsistentHashingMap 一致性hash映射
type ConsistentHashingMap struct {
	mu            sync.RWMutex     // 读写锁
	config        *Config          // 配置信息
	hashCircle    []int            // 哈希环
	hashMap       map[int]string   // 哈希环到节点的映射
	virtualNodes  map[string]int   // 节点到虚拟节点数量的映射
	nodeCounts    map[string]int64 // 节点负载统计
	totalRequests int64            // 总请求数
}

// Option 一致性hash配置
type Option func(*ConsistentHashingMap)

// WithConfig 设置配置
func WithConfig(config *Config) Option {
	return func(m *ConsistentHashingMap) {
		m.config = config
	}
}

// New 创建一致性hash实例
func New(opts ...Option) *ConsistentHashingMap {
	m := &ConsistentHashingMap{
		config:       DefaultConfig,
		hashMap:      make(map[int]string),
		virtualNodes: make(map[string]int),
		nodeCounts:   make(map[string]int64),
	}

	for _, opt := range opts {
		opt(m)
	}

	// 开启负载均衡
	m.startBalancer()

	return m
}

// Add 添加节点
func (m *ConsistentHashingMap) Add(nodes ...string) error {
	if len(nodes) == 0 {
		return errors.New("no nodes provided")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, node := range nodes {
		if node == "" {
			continue
		}

		// 为节点添加虚拟节点
		m.addVirutalNodes(node, m.config.DefaultVirtualNodes)
	}

	// 重新排序
	sort.Ints(m.hashCircle)
	return nil
}

// Get 获取节点
func (m *ConsistentHashingMap) Get(key string) string {
	if key == "" {
		return ""
	}

	// 加读锁
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.hashCircle) == 0 {
		return ""
	}

	// 计算key的哈希值
	hash := int(m.config.HashFunc([]byte(key)))

	// 二分查找,顺时针寻找大于等于hash值的一个值的索引
	idx := sort.Search(len(m.hashCircle), func(i int) bool {
		return m.hashCircle[i] >= hash
	})

	// 处理边界情况
	if idx == len(m.hashCircle) {
		idx = 0
	}

	node := m.hashMap[m.hashCircle[idx]] // 获取节点
	count := m.nodeCounts[node]          // 获取节点负载
	m.nodeCounts[node] = count + 1       // 节点负载统计+1
	atomic.AddInt64(&m.totalRequests, 1) //  总请求数+1
	return node
}

// Remove 移除节点
func (m *ConsistentHashingMap) Remove(node string) error {
	if node == "" {
		return errors.New("invalid node,remove fail")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 移除节点的所有虚拟节点
	replicas := m.virtualNodes[node]
	if replicas == 0 {
		return fmt.Errorf("node %s not found", node)
	}

	for i := range replicas {
		hash := int(m.config.HashFunc([]byte(fmt.Sprintf("%s-%d", node, i))))
		delete(m.hashMap, hash)
		for j := 0; j < len(m.hashCircle); j++ {
			if m.hashCircle[j] == hash {
				m.hashCircle = append(m.hashCircle[:j], m.hashCircle[j+1:]...) // 从哈希环删除虚拟节点
				break
			}
		}
	}
	delete(m.virtualNodes, node)
	delete(m.nodeCounts, node)
	return nil
}

// addVirutalNodes 添加节点的虚拟节点
func (m *ConsistentHashingMap) addVirutalNodes(node string, counts int) {
	for i := range counts {
		hash := int(m.config.HashFunc([]byte(fmt.Sprintf("%s-%d", node, i))))
		m.hashCircle = append(m.hashCircle, hash) // 加入哈希环
		m.hashMap[hash] = node                    // 哈希环到节点的映射
	}
	m.virtualNodes[node] = counts // 节点到虚拟节点数量的映射
}

// 将checkAndReBalance移到单独的goroutine中
func (m *ConsistentHashingMap) startBalancer() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.checkAndReBalance()
		}
	}()
}

// checkAndReBalance 检查并重新平衡
func (m *ConsistentHashingMap) checkAndReBalance() {
	if atomic.LoadInt64(&m.totalRequests) < 1000 {
		return // 样本太少，不进行调整
	}

	// 计算负载情况
	avgLoad := float64(m.totalRequests) / float64(len(m.virtualNodes))
	var maxDiff float64

	for _, count := range m.nodeCounts {
		diff := math.Abs(float64(count) - avgLoad)
		if diff/avgLoad > maxDiff {
			maxDiff = diff / avgLoad
		}
	}

	// 如果负载不均衡度超过阈值，调整虚拟节点
	if maxDiff > m.config.LoadBalanceThreshold {
		m.reBalanceNodes()
	}
}

// reBalanceNodes 重新平衡节点
func (m *ConsistentHashingMap) reBalanceNodes() {
	m.mu.Lock()
	defer m.mu.Unlock()

	avgLoad := float64(m.totalRequests) / float64(len(m.virtualNodes))

	// 调整每个节点的虚拟节点数量
	for node, count := range m.nodeCounts {
		currentVirtualNodes := m.virtualNodes[node] // 当前节点的虚拟节点数量
		loadRatio := float64(count) / avgLoad       // 负载比率

		var newVirtualCounts int // 新虚拟节点数量
		if loadRatio > 1 {
			// 负载过高，减少虚拟节点
			newVirtualCounts = int(float64(currentVirtualNodes) / loadRatio)
		} else {
			// 负载过低，增加虚拟节点
			newVirtualCounts = int(float64(currentVirtualNodes) * (2 - loadRatio))
		}
		// 确保在限制范围内
		if newVirtualCounts < m.config.MinVirtualNodes {
			newVirtualCounts = m.config.MinVirtualNodes
		}
		if newVirtualCounts > m.config.MaxVirtualNodes {
			newVirtualCounts = m.config.MaxVirtualNodes
		}

		if newVirtualCounts != currentVirtualNodes {
			// 移除之前的虚拟节点
			if err := m.Remove(node); err != nil {
				continue // 如果移除失败，跳过这个节点
			}
			// 添加新的虚拟节点
			m.addVirutalNodes(node, newVirtualCounts)
		}
	}

	// 重置计数器
	for node := range m.nodeCounts {
		m.nodeCounts[node] = 0
	}
	atomic.StoreInt64(&m.totalRequests, 0)

	// 重新排序
	sort.Ints(m.hashCircle)
}

// GetStats 获取负载统计信息
func (m *ConsistentHashingMap) GetStats() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]float64)
	total := atomic.LoadInt64(&m.totalRequests)
	if total == 0 {
		return stats
	}
	for node, count := range m.nodeCounts {
		stats[node] = float64(count) / float64(total)
	}
	return stats
}
