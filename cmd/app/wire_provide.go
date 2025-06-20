package app

import (
	"context"
	"time"

	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"

	lcache "github.com/zuozikang/cache"
	"github.com/zuozikang/cache/consts"
	"github.com/zuozikang/cache/db"
)

// ProvideServer wire
func ProvideServer(addr string) (*lcache.Server, error) {
	return lcache.NewServer(addr, consts.DefaultClientName,
		lcache.WithEtcdEndpoints([]string{"localhost:2379"}),
		lcache.WithDialTimeout(5*time.Second))
}

// ProvideGroup wire
func ProvideGroup() *lcache.Group {
	return lcache.NewGroup("test", 2<<20, lcache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return db.Get(context.Background(), key)
		}),
	)
}

// ProvideNacosClientParam wire
func ProvideNacosClientParam(config *Config) (vo.NacosClientParam, error) {
	// 构建ServerConfig
	sc := []constant.ServerConfig{{
		IpAddr: config.NacosServer.IpAddr,
		Port:   config.NacosServer.Port,
		Scheme: config.NacosServer.Scheme,
	}}

	// 构建ClientConfig
	cc := constant.ClientConfig{
		NamespaceId:         config.NacosClient.NamespaceId,
		TimeoutMs:           config.NacosClient.TimeoutMs,
		NotLoadCacheAtStart: config.NacosClient.NotLoadCacheAtStart,
		LogDir:              config.NacosClient.LogDir,
		CacheDir:            config.NacosClient.CacheDir,
		LogLevel:            config.NacosClient.LogLevel,
	}

	return vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: sc,
	}, nil
}
