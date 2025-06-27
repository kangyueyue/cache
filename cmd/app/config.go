package app

import (
	"github.com/kangyueyue/road"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/zuozikang/cache/db"

	"github.com/BurntSushi/toml"
)

// Config nacos配置
type Config struct {
	DbConfig    *db.DBConfig   `toml:"db-config"`    // 数据库配置
	RedisConfig *redis.Options `toml:"redis-config"` // redis配置
}

// NewConfig wire
func NewConfig(_ *road.Road) (*Config, error) {
	// 加载默认配置
	cfg := DefaultConfig()
	// 读取nacos配置，覆盖默认配置
	if _, err := toml.Decode(viper.GetString("config.conf"), &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
