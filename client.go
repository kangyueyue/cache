package kamacache

import (
	"context"
	"fmt"
	"gitee.com/messizuo/kama-cache-go/pb"
	re "gitee.com/messizuo/kama-cache-go/retry"
	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

// Client 定义了缓存客户端接口，实现Peer接口
type Client struct {
	addr        string
	svcName     string
	etcdCli     *clientv3.Client
	coon        *grpc.ClientConn
	grpcCli     pb.KamaCacheClient
	retryConfig *re.RetryConfig
}

var _ Peer = (*Client)(nil)

// NewClient 创建缓存客户端
func NewClient(addr, svcName string, etcdCli *clientv3.Client) (*Client, error) {
	var err error
	// 注册中心 etcd为空
	if etcdCli == nil {
		// 创建默认的服务endpoint
		etcdCli, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create etcd client: %v", err)
		}
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %v", err)
	}

	grpcClient := pb.NewKamaCacheClient(conn)

	retryClient := re.NewRetryConfig()

	client := &Client{
		addr:        addr,
		svcName:     svcName,
		etcdCli:     etcdCli,
		coon:        conn,
		grpcCli:     grpcClient,
		retryConfig: retryClient,
	}
	return client, nil
}

// Get 获取缓存
func (c *Client) Get(group string, key string) ([]byte, error) {
	// 创建超时ctx
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var resp *pb.ResponseForGet
	var err error
	// 发grpc请求，重试3次
	err = retry.Do(func() error {
		resp, err = c.grpcCli.Get(ctx, &pb.Request{
			Group: group,
			Key:   key,
		})
		return err
	},
		retry.Attempts(c.retryConfig.MaxAttempts),
		retry.Delay(c.retryConfig.Delay),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(c.retryConfig.RetryIfFn),
		retry.OnRetry(func(n uint, err error) {
			logrus.Infof("retry get cache from peers, err: %v,count: %d", err, n+1)
		}),
	)
	if err != nil {
		logrus.Errorf("use grpc to get cache from peers err: %v", err)
		return nil, fmt.Errorf("use grpc to get cache from peers err: %w", err)
	}
	return resp.GetValue(), nil
}

// Set 设置缓存
func (c Client) Set(ctx context.Context, group string, key string, value []byte) error {
	// 发grpc请求
	var err error

	// 发grpc请求，重试3次
	err = retry.Do(func() error {
		_, err = c.grpcCli.Set(ctx, &pb.Request{
			Group: group,
			Key:   key,
			Value: value,
		})
		return err
	},
		retry.Attempts(c.retryConfig.MaxAttempts),
		retry.Delay(c.retryConfig.Delay),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(c.retryConfig.RetryIfFn),
		retry.OnRetry(func(n uint, err error) {
			logrus.Infof("retry get cache from peers, err: %v,count: %d", err, n+1)
		}),
	)
	if err != nil {
		return fmt.Errorf("use grpc to set cache to peers err: %w", err)
	}
	return nil
}

// Delete 删除缓存
func (c Client) Delete(group string, key string) (bool, error) {
	// 创建超时ctx
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var err error
	var resp *pb.ResponseForDelete
	err = retry.Do(func() error {
		resp, err = c.grpcCli.Delete(ctx, &pb.Request{
			Group: group,
			Key:   key,
		})
		return err
	},
		retry.Attempts(c.retryConfig.MaxAttempts),
		retry.Delay(c.retryConfig.Delay),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(c.retryConfig.RetryIfFn),
		retry.OnRetry(func(n uint, err error) {
			logrus.Infof("retry get cache from peers, err: %v,count: %d", err, n+1)
		}),
	)

	if err != nil {
		return false, fmt.Errorf("use grpc to delete cache from peers err: %w", err)
	}

	return resp.GetValue(), nil
}

// Close 关闭客户端资源
func (c Client) Close() error {
	if c.coon != nil {
		return c.coon.Close()
	}
	return nil
}
