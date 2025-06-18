package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdConnect(t *testing.T) {
	// 测试etcd
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
		return
	}

	// 创建一个简单的连接
	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
	defer cancel()
	_, err = cli.Get(ctx, "test_key")
	if err != nil {
		t.Fatalf("get error: %v", err)
		return
	}
	t.Logf("etcd connect success")
}

func TestEtcdPut(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	defer cli.Close()
	// put
	_, err = cli.Put(context.TODO(), "foo", "bar")
	if err != nil {
		t.Fatalf("put error: %v", err)
	}
}

func TestEtcdGet(t *testing.T) {
	// 测试etcd
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	defer cli.Close()
	// put
	_, err = cli.Put(context.TODO(), "foo", "bar1")
	if err != nil {
		t.Fatalf("err : %s", err)
	}
	// get
	resp, err := cli.Get(context.TODO(), "foo")
	if err != nil {
		t.Logf("err: %s", err)
	}
	for _, ev := range resp.Kvs {
		t.Logf("获取到的kv:%s : %s\n", ev.Key, ev.Value)
	}
	resp, err = cli.Get(context.TODO(), "foo1")
	assert.Equal(t, err != nil, false)
	assert.Equal(t, resp.Count, int64(0)) // 必须找不到 foo1
}
