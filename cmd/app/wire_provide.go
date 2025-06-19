package app

import (
	"context"
	"time"

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

// ProvidePicker wire
func ProvidePicker(addr string) (*lcache.ClientPicker, error) {
	return lcache.NewClientPicker(addr)
}

// ProvideGroup wire
func ProvideGroup() *lcache.Group {
	return lcache.NewGroup("test", 2<<20, lcache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return db.Get(context.Background(), key)
		}),
	)
}
