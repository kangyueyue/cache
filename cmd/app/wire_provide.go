package app

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	lcache "github.com/zuozikang/cache"
	"github.com/zuozikang/cache/consts"
	"github.com/zuozikang/cache/db"
)

// ProvideServer wire
func ProvideServer(addr int) (*lcache.Server, error) {
	return lcache.NewServer(addr, consts.DefaultClientName,
		lcache.WithEtcdEndpoints([]string{"localhost:2379"}),
		lcache.WithDialTimeout(5*time.Second))
}

// ProvideGroup wire
func ProvideGroup(store *db.Store) *lcache.Group {
	return lcache.NewGroup("test", 2<<20, lcache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return store.Get(context.Background(), key)
		}),
	)
}

// ProvideRedisClient wire
func ProvideRedisClient(cfg *Config) *redis.Client {
	return redis.NewClient(cfg.RedisConfig)
}

// ProvideStore wire
func ProvideStore(cfg *Config) (*db.Store, error) {
	return db.NewStore(cfg.DbConfig)
}
