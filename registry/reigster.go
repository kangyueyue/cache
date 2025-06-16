package registry

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"net"
	"time"
)

// Config 服务配置
type Config struct {
	EndPoints   []string
	DialTimeout time.Duration
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		EndPoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	}
}

// Register 注册服务
func Register(srcName, addr string, stopCh <-chan error) error {
	defaultConfig := DefaultConfig()
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   defaultConfig.EndPoints,
		DialTimeout: defaultConfig.DialTimeout,
	})
	if err != nil {
		return fmt.Errorf("connect etcd failed, err: %w", err)
	}

	localIp, err := getLocalIp()
	if err != nil {
		err1 := cli.Close()
		if err1 != nil {
			logrus.Errorf("close etcd client failed, err: %v", err1)
		}
		return fmt.Errorf("get local ip failed, err: %w", err)
	}
	if addr[0] == ':' {
		addr = fmt.Sprintf("%s%s", localIp, addr)
	}

	// 创建租约
	lease, err := cli.Grant(context.Background(), 10)
	if err != nil {
		err1 := cli.Close()
		if err1 != nil {
			logrus.Errorf("close etcd client failed, err: %v", err1)
		}
		return fmt.Errorf("create lease failed, err: %w", err)
	}

	// 注册服务
	key := fmt.Sprintf("/services/%s/%s", srcName, addr)
	_, err = cli.Put(context.Background(), key, addr, clientv3.WithLease(lease.ID))
	if err != nil {
		err1 := cli.Close()
		if err1 != nil {
			logrus.Errorf("close etcd client failed, err: %v", err1)
		}
		return fmt.Errorf("failed to put key-value to etcd: %v", err)
	}

	// 保持租约
	keepActiveCh, err := cli.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		err1 := cli.Close()
		if err1 != nil {
			logrus.Errorf("close etcd client failed, err: %v", err1)
		}
		return fmt.Errorf("failed to keep alive: %v", err)
	}

	// 处理租约续期和服务注销
	go func() {
		defer func() {
			err1 := cli.Close()
			if err1 != nil {
				logrus.Errorf("close etcd client failed, err: %v", err1)
			}
		}()
		select {
		case <-stopCh:
			// 服务注销，撤销租约
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			cli.Revoke(ctx, lease.ID)
			defer cancel()
			return
		case resp, ok := <-keepActiveCh:
			if !ok {
				logrus.Infof("keep alive channel closed")
				return
			}
			logrus.Infof("successfully keep alive resp:%v", resp.ID)
		}
	}()

	logrus.Infof("Service registered: %s at %s", srcName, addr)
	return nil
}

// 获取本地ip
func getLocalIp() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no valid local IP found")
}
