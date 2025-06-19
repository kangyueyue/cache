package kamacache

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zuozikang/cache/consistenthash"
	"github.com/zuozikang/cache/consts"
	"github.com/zuozikang/cache/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// PeerPicker 定义了peer选择器的接口
type PeerPicker interface {
	PickPeer(key string) (peer Peer, ok, self bool) // 选择器
	PrintPeers()                                    // 打印当前已发现的节点
	Close() error
}

// Peer 定义了peer接口
type Peer interface {
	Get(group string, key string) ([]byte, error)
	Set(ctx context.Context, group string, key string, value []byte) error
	Delete(group string, key string) (bool, error)
	Close() error
}

// ClientPicker 缓存客户端，实现了PeerPicker接口
type ClientPicker struct {
	selfAddr          string
	svcName           string
	mu                sync.RWMutex
	etcdCli           *clientv3.Client
	clients           map[string]*Client
	consistentHashing *consistenthash.ConsistentHashingMap
	ctx               context.Context
	cancel            context.CancelFunc
}

// PickerOption 缓存客户端的配置项
type PickerOption func(*ClientPicker)

// DefaultPickerOptions 默认选项
func DefaultPickerOptions() []PickerOption {
	return []PickerOption{}
}

// WithServiceName 设置服务名
func WithServiceName(name string) PickerOption {
	return func(peer *ClientPicker) {
		peer.svcName = name
	}
}

// NewClientPicker 创建新的ClientPicker实例
func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error) {
	ctx, cancel := context.WithCancel(context.Background())
	picker := &ClientPicker{
		selfAddr:          addr,
		svcName:           consts.DefaultClientName,
		clients:           make(map[string]*Client),
		consistentHashing: consistenthash.New(), // 一致性哈希
		ctx:               ctx,
		cancel:            cancel,
	}

	for _, opt := range opts {
		opt(picker)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   registry.DefaultConfig().EndPoints,
		DialTimeout: registry.DefaultConfig().DialTimeout,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create etcd client: %v", err)
	}
	picker.etcdCli = cli

	// 启动服务发现
	if err = picker.startServiceDiscovery(); err != nil {
		cancel()
		err1 := cli.Close()
		if err1 != nil {
			log.Fatalf("failed to close etcd client: %v", err)
		}
		return nil, fmt.Errorf("failed to start service discovery: %v", err)
	}

	return picker, nil
}

// PrintPeers 打印当前已发现的节点（仅用于调试）
func (p *ClientPicker) PrintPeers() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	logrus.Infof("当前已发现的节点:")
	for addr := range p.clients {
		logrus.Infof("- %s", addr)
	}
}

// startServiceDiscovery 启动服务发现
func (c *ClientPicker) startServiceDiscovery() error {
	// 全量更新
	if err := c.fetchAllServices(); err != nil {
		return fmt.Errorf("failed to fetch all services: %v", err)
	}

	// 增量更新
	go c.watchServiceChanges()
	return nil
}

// fetchAllServices 获取所有服务实例
func (p *ClientPicker) fetchAllServices() error {
	ctx, cancelFunc := context.WithTimeout(p.ctx, 3*time.Second)
	defer cancelFunc()

	resp, err := p.etcdCli.Get(ctx, "/services/"+p.svcName, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to get services: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, kv := range resp.Kvs {
		addr := string(kv.Value)
		if addr != "" && addr != p.selfAddr {
			p.set(addr)
			logrus.Infof("Discover service at : %s", addr)
		}
	}
	return nil
}

// PickPeer 选择peer节点
func (c *ClientPicker) PickPeer(key string) (peer Peer, ok, self bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 一致性哈希，consHash
	if addr := c.consistentHashing.Get(key); addr != "" {
		if client, ok := c.clients[addr]; ok {
			logrus.Infof("Picked peer %s for key %s", addr, key)
			return client, true, addr == c.selfAddr
		}
	}
	logrus.Infof("No peer found for key %s", key)

	return nil, false, false
}

// Close 关闭ClientPicker
func (c *ClientPicker) Close() error {
	c.cancel()
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	// 关闭clients
	for addr, client := range c.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close client %s: %v", addr, err))
		}
	}

	if err := c.etcdCli.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close etcd client: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors while closing: %v", errs)
	}
	return nil
}

// set 添加服务实例
func (c *ClientPicker) set(addr string) {
	if client, err := NewClient(addr, c.svcName, c.etcdCli); err == nil {
		// 一致性哈希，consHash
		err = c.consistentHashing.Add(addr)
		if err != nil {
			logrus.Errorf("Failed to add %s to consistent hash: %v", addr, err)
		}
		c.clients[addr] = client // add client
		logrus.Infof("add client successfully: %s", addr)
	} else {
		logrus.Errorf("Failed to create client for %s: %v", addr, err)
	}
}

// watchServiceChanges 监听服务实例变化
func (c *ClientPicker) watchServiceChanges() {
	watcher := clientv3.NewWatcher(c.etcdCli)
	watchChan := watcher.Watch(c.ctx, "/services/"+c.svcName, clientv3.WithPrefix())

	for {
		select {
		case <-c.ctx.Done():
			err := watcher.Close()
			if err != nil {
				logrus.Errorf("failed to close etcd watcher: %v", err)
			}
			return
		case resp := <-watchChan:
			c.handleWatchEvents(resp.Events)
		}
	}
}

// handleWatchEvents 处理watch事件
func (c *ClientPicker) handleWatchEvents(events []*clientv3.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, event := range events {
		addr := string(event.Kv.Value)
		if addr == c.selfAddr {
			continue
		}

		switch event.Type {
		case clientv3.EventTypePut: // put
			if _, ok := c.clients[addr]; !ok {
				c.set(addr)
				logrus.Infof("New service discovered at %s", addr)
			}
		case clientv3.EventTypeDelete: // delete
			if client, ok := c.clients[addr]; ok {
				err := client.Close()
				if err != nil {
					logrus.Errorf("Failed to close client for %s: %v", addr, err)
				}
				c.remove(addr)
				logrus.Infof("Service %s is removed", addr)
			}
		}
	}
}

// remove 移除服务实例
func (c *ClientPicker) remove(addr string) {
	// 一致性哈希，consHash
	err := c.consistentHashing.Remove(addr)
	if err != nil {
		logrus.Errorf("Failed to remove %s from consistent hash: %v", addr, err)
	}
	delete(c.clients, addr) // remove client
}

// parseAddrFromKey 从etcd key中解析地址
func parseAddrFromKey(key, svcName string) string {
	prefix := fmt.Sprintf("/services/%s/", svcName)
	if strings.HasPrefix(key, prefix) {
		return strings.TrimPrefix(key, prefix)
	}
	return ""
}
