package app

import (
	"bytes"

	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	lcache "github.com/zuozikang/cache"
	"github.com/zuozikang/cache/consts"
)

// InitNacos 初始化配置
func (a *App) InitNacos() {
	var (
		err     error
		content string
	)
	// 从nacos读取配置
	content, err = a.nacosClient.GetConfig(vo.ConfigParam{
		DataId: consts.DataId,
		Group:  consts.GROUP,
	})
	// 写入到本地，缓存配置
	lcache.CreateConfigCache(content)

	if err != nil {
		logrus.Fatalf("从nacos中读取配置文件失败: %v", err)
		panic(err)
	}
	viper.SetConfigType("yml")
	if err = viper.ReadConfig(bytes.NewBuffer([]byte(content))); err != nil {
		logrus.Fatalf("读取配置文件失败: %v", err)
		panic(err)
	}
	// 监听
	err = a.nacosClient.ListenConfig(vo.ConfigParam{
		DataId: consts.DataId,
		Group:  consts.GROUP,
		OnChange: func(namespace, group, dataId, data string) {
			logrus.Info("检测到配置变更，重新加载配置")
			// 重新加载配置
			if err = viper.ReadConfig(bytes.NewBuffer([]byte(data))); err != nil {
				logrus.Errorf("重新加载配置失败: %v", err)
				return
			}
			lcache.CreateConfigCache(data) // 写入到本地
			// 可以在这里添加配置变更后的处理逻辑
			logrus.Info("配置重新加载成功")
		},
	})

	if err != nil {
		logrus.Warnf("配置监听设置失败: %v", err)
	}
}
