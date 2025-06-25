package app

import (
	"github.com/redis/go-redis/v9"
	"github.com/zuozikang/cache/db"
	"os"

	"github.com/BurntSushi/toml"
)

// Config nacos配置
type Config struct {
	NacosServer *NacosServer   `toml:"nacos_server"` // nacos服务端配置
	NacosClient *NacosClient   `toml:"nacos_client"` // nacos客户端配置
	DbConfig    *db.DBConfig   `toml:"db-config"`    // 数据库配置
	RedisConfig *redis.Options `toml:"redis-config"` // redis配置
}

// NacosServer nacos服务端配置
type NacosServer struct {
	IpAddr string `toml:"ip_addr"`
	Port   uint64 `toml:"port"`
	Scheme string `toml:"scheme"`
}

// NacosClient nacos客户端配置
type NacosClient struct {
	NamespaceId         string `toml:"namespace_id"`
	TimeoutMs           uint64 `toml:"timeout_ms"`
	NotLoadCacheAtStart bool   `toml:"not_load_cache_at_start"`
	LogDir              string `toml:"log_dir"`
	CacheDir            string `toml:"cache_dir"`
	LogLevel            string `toml:"log_level"`
}

// NewConfig wire
func NewConfig() (*Config, error) {
	// 读取TOML配置文件
	config := Config{}
	file, err := os.ReadFile("config/cache.conf")
	if err != nil {
		return nil, err
	}
	if _, err = toml.Decode(string(file), &config); err != nil {
		return nil, err
	}
	return &config, nil
}
