package app

import (
	"github.com/redis/go-redis/v9"
	"github.com/zuozikang/cache/db"
)

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		&db.DBConfig{},
		&redis.Options{
		},
	}
}
