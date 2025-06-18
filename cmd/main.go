package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	lcache "github.com/zuozikang/cache"
	"github.com/zuozikang/cache/consts"
	"github.com/zuozikang/cache/db"
	logs "github.com/zuozikang/cache/logurs"
)

func main() {
	logs.InitLog()
	app := &cli.App{
		Name:  "zuo-cache",
		Usage: "KamaCache-go-zuo",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "port",
				Value: 8001,
				Usage: "端口",
			},
			&cli.StringFlag{
				Name:  "node",
				Value: "A",
				Usage: "节点标识符",
			},
		},
		Action: func(c *cli.Context) error {
			logrus.Infof("cmd start")

			port := c.Int("port")
			nodeId := c.String("node")

			addr := fmt.Sprintf(":%d", port)
			logrus.Infof("[节点%s] 启动，地址: %s", nodeId, addr)

			// 创建节点
			server, err := lcache.NewServer(addr, consts.DefaultClientName,
				lcache.WithEtcdEndpoints([]string{"localhost:2379"}),
				lcache.WithDialTimeout(5*time.Second))
			if err != nil {
				logrus.Fatalf("failed to create server: %v", err)
				return err
			}

			// 创建节点选择器
			picker, err := lcache.NewClientPicker(addr)
			if err != nil {
				logrus.Fatalf("failed to create client picker: %v", err)
				return err
			}

			// 创建DB
			err = db.InitDB()
			if err != nil {
				logrus.Fatalf("failed to init db: %v", err)
				return err
			}

			// 创建缓存组
			group := lcache.NewGroup("test", 2<<20, lcache.GetterFunc(
				func(ctx context.Context, key string) ([]byte, error) {
					return db.Get(context.Background(), key)
				}),
			)

			// 注册节点选择器
			group.RegisterPeers(picker)

			// 异步启动节点
			go func() {
				logrus.Infof("[节点%s] 启动服务", nodeId)
				if err := server.Start(); err != nil {
					logrus.Fatalf("failed to start server: %v", err)
				}
			}()

			// 等待节点注册完成
			logrus.Printf("[节点%s] 等待节点注册完成", nodeId)
			time.Sleep(5 * time.Second)

			ctx := context.Background()

			// 设置本节点的特定键值对
			localKey := fmt.Sprintf("key_%s", nodeId)
			localValue := []byte(fmt.Sprintf("这是节点%s的数据", nodeId))
			DbValue := []byte(fmt.Sprintf("节点%s的db数据", nodeId))

			logrus.Infof("\n=== 节点%s：设置本地数据 ===\n", nodeId)
			err = group.Set(ctx, localKey, localValue)
			if err != nil {
				logrus.Fatal("设置本地数据失败:", err)
			}

			logrus.Infof("\n=== 节点%s：设置db数据 ===\n", nodeId)
			err = db.Set(ctx, localKey, DbValue)
			if err != nil {
				logrus.Fatal("设置db数据失败:", err)
			}

			// 等待其他节点完成设置
			logrus.Infof("[节点%s] 等待其他节点准备就绪...", nodeId)
			time.Sleep(30 * time.Second)

			// 打印当前已发现的节点
			picker.PrintPeers()

			// 获取本地数据
			logrus.Infof("\n=== 节点%s：获取本地数据 ===\n", nodeId)
			logrus.Infof("直接查询本地缓存...\n")

			if val, err := group.Get(ctx, localKey); err != nil {
				logrus.Fatalf("节点%s: 获取本地键失败: %v\n", nodeId, err)
			} else {
				logrus.Infof("节点%s: 获取本地键 %s 成功: %s\n", nodeId, localKey, val.String())
			}

			// 打印缓存统计信息
			stats := group.Stats()
			logrus.Infof("获取本地缓存之后的缓存统计: %+v\n", stats)

			// 测试获取其他节点的数据
			otherNodes := []string{"key_A", "key_B", "key_C", "key_D"}
			for _, key := range otherNodes {
				if key == localKey {
					continue // 跳过本节点的键
				}
				logrus.Infof("\n=== 节点%s：尝试获取远程数据 %s ===\n", nodeId, key)
				logrus.Infof("[节点%s] 开始查找键 %s 的远程节点", nodeId, key)
				if val, err := group.Get(ctx, key); err == nil {
					logrus.Infof("节点%s: 获取远程键 %s 成功: %s\n", nodeId, key, val.String())
					// 打印stats
					logrus.Infof("获取节点%s缓存之后的缓存统计: %+v\n", key, group.Stats())
				} else {
					logrus.Infof("节点%s: 获取远程键失败: %v\n", nodeId, err)
				}
			}

			select {}
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}
