package app

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"time"

	"github.com/urfave/cli/v2"
	lcache "github.com/zuozikang/cache"
	"github.com/zuozikang/cache/db"
)

// App 应用
type App struct {
	addr   string               // 地址
	server *lcache.Server       // 服务
	picker *lcache.ClientPicker // 节点选择器
	group  *lcache.Group        // 分组
	log    *logrus.Logger       // 日志
}

// NewApp 创建应用
func NewApp(server *lcache.Server,
	picker *lcache.ClientPicker,
	group *lcache.Group,
) *App {
	return &App{
		server: server,
		picker: picker,
		group:  group,
		log:    logrus.New(),
	}
}

// Server 启动
func (a *App) Server(c *cli.Context) error {
	a.log.Infof("cmd start")

	port := c.Int("port")
	nodeId := c.String("node")

	addr := fmt.Sprintf(":%d", port)
	a.log.Infof("[节点%s] 启动，地址: %s", nodeId, addr)

	// Init db
	err := db.InitDB()
	if err != nil {
		a.log.Errorf("InitDB err: %v", err)
		return err
	}

	// 注册节点选择器
	a.group.RegisterPeers(a.picker)

	err = a.startServer(nodeId)
	if err != nil {
		return err
	}

	// 等待节点注册完成
	a.log.Printf("[节点%s] 等待节点注册完成", nodeId)
	time.Sleep(5 * time.Second)

	// logic 执行逻辑
	err = a.logic(nodeId)
	if err != nil {
		return err
	}
	return nil
}

// Close 关闭
func (a *App) Close() error {
	var err error
	a.server.Stop() // 停止服务
	a.log.Infof("server closed")
	err = a.group.Close() // 关闭分组
	if err != nil {
		a.log.Errorf("group.Close err: %v", err)
		return err
	}
	a.log.Infof("group closed")
	err = a.picker.Close() // 关闭选择器
	if err != nil {
		a.log.Errorf("picker.Close err: %v", err)
		return err
	}
	a.log.Infof("picker closed")
	return nil
}

// logic 执行逻辑
func (a *App) logic(nodeId string) error {
	var (
		ctx = context.Background()
		err error
	)
	// 设置本节点的特定键值对
	localKey := fmt.Sprintf("key_%s", nodeId)
	localValue := []byte(fmt.Sprintf("这是节点%s的数据", nodeId))
	DbValue := []byte(fmt.Sprintf("节点%s的db数据", nodeId))
	a.log.Infof("\n=== 节点%s：设置本地数据 ===\n", nodeId)
	err = a.group.Set(ctx, localKey, localValue)
	if err != nil {
		a.log.Fatal("设置本地数据失败:", err)
	}

	a.log.Infof("\n=== 节点%s：设置db数据 ===\n", nodeId)
	err = db.Set(ctx, localKey, DbValue)
	if err != nil {
		a.log.Fatal("设置db数据失败:", err)
	}

	// 等待其他节点完成设置
	a.log.Infof("[节点%s] 等待其他节点准备就绪...", nodeId)
	time.Sleep(30 * time.Second)

	// 打印当前已发现的节点
	a.picker.PrintPeers()

	err = a.getLocalCache(ctx, localKey, nodeId)
	if err != nil {
		return err
	}
	err = a.getOtherCache(ctx, localKey, nodeId)
	if err != nil {
		return err
	}
	return nil
}

// startServer 异步启动服务
func (a *App) startServer(nodeId string) error {
	var err error
	a.log.Infof("[节点%s] 启动服务", nodeId)
	if err = a.server.Start(); err != nil {
		a.log.Fatalf("failed to start server: %v", err)
	}
	return err
}

// getLocalCache 获取本地缓存
func (a *App) getLocalCache(ctx context.Context, localKey, nodeId string) error {
	// 获取本地数据
	a.log.Infof("\n=== 节点%s：获取本地数据 ===\n", nodeId)
	a.log.Infof("直接查询本地缓存...\n")

	if val, err := a.group.Get(ctx, localKey); err != nil {
		a.log.Fatalf("节点%s: 获取本地键失败: %v\n", nodeId, err)
		return err
	} else {
		a.log.Infof("节点%s: 获取本地键 %s 成功: %s\n", nodeId, localKey, val.String())
	}

	// 打印缓存统计信息
	stats := a.group.Stats()
	a.log.Infof("获取本地缓存之后的缓存统计: %+v\n", stats)
	return nil
}

// getOtherCache 获取其他节点缓存
func (a *App) getOtherCache(ctx context.Context, localKey, nodeId string) error {
	otherNodes := []string{"key_A", "key_B", "key_C", "key_D"}
	for _, key := range otherNodes {
		if key == localKey {
			continue // 跳过本节点的键
		}
		a.log.Infof("\n=== 节点%s：尝试获取远程数据 %s ===\n", nodeId, key)
		a.log.Infof("[节点%s] 开始查找键 %s 的远程节点", nodeId, key)
		if val, err := a.group.Get(ctx, key); err == nil {
			a.log.Infof("节点%s: 获取远程键 %s 成功: %s\n", nodeId, key, val.String())
			// 打印stats
			a.log.Infof("获取节点%s缓存之后的缓存统计: %+v\n", key, a.group.Stats())
		} else {
			a.log.Infof("节点%s: 获取远程键失败: %v\n", nodeId, err)
			return err
		}
	}
	return nil
}
