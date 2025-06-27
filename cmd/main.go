package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/urfave/cli/v2"
	"github.com/zuozikang/cache/cmd/app"
	logs "github.com/zuozikang/cache/logurs"
)

func main() {
	logs.InitLog() // 初始化日志
	appCmd := &cli.App{
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
		Action: run,
	}
	err := appCmd.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

// run 启动服务
func run(c *cli.Context) error {
	cacheApp, err := app.InitializeApp(c.Int("port"), "./config/cache.conf")
	if err != nil {
		return err
	}
	// 启动服务
	if err = cacheApp.Server(c); err != nil {
		panic(err)
	}
	// 关闭资源
	defer func() {
		err = cacheApp.Close()
		if err != nil {
			panic(err)
		}
		logrus.Infof("[节点%s] 服务已关闭", c.String("node"))
	}()

	// 阻塞，保持长期运行
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM) // 监听ctrl+c和kill命令

	// 阻塞等待
	<-sigCh
	logrus.Infof("接收到终止信号，开始关闭......")
	return nil
}
