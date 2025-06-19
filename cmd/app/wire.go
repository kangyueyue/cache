//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package app

import (
	"github.com/google/wire"
	lcache "github.com/zuozikang/cache"
)

// AppSet 依赖
var AppSet = wire.NewSet(
	NewApp,
)

// 绑定接口和实现类
var PickerSet = wire.NewSet(
	lcache.NewClientPicker,
	lcache.DefaultPickerOptions,
	wire.Bind(new(lcache.PeerPicker), new(*lcache.ClientPicker)),
)

// ProviderSet 依赖
var ProviderSet = wire.NewSet(
	ProvideServer,
	ProvideGroup,
)

// InitializeApp 初始化
func InitializeApp(addr string) (*App, error) {
	wire.Build(
		AppSet,
		PickerSet,
		ProviderSet,
	)
	return &App{}, nil // 返回值没有实际意义，只需符合接口即可
}
