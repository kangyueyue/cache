package consistenthash

import "hash/crc32"

// Config 一致性hash配置
type Config struct {
	// 每个真实节点对应的虚拟节点数
	DefaultVirtualNodes int
	// 最小虚拟节点数
	MinVirtualNodes int
	// 最大虚拟节点数
	MaxVirtualNodes int
	// 哈希函数
	HashFunc func(data []byte) uint32
	// 负载均衡阈值，超过此值触发虚拟节点调整
	LoadBalanceThreshold float64
}

var DefaultConfig = &Config{
	DefaultVirtualNodes:  50,                 // 每个真实节点对应 50 个虚拟节点
	MinVirtualNodes:      10,                 // 最小虚拟节点数
	MaxVirtualNodes:      200,                // 最大虚拟节点数
	HashFunc:             crc32.ChecksumIEEE, // 默认使用 CRC32 算法
	LoadBalanceThreshold: 0.25,               // 25% 的负载不均衡度触发调整
}
