//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package app

import "github.com/google/wire"

// AppSet 依赖
var AppSet = wire.NewSet(
	NewApp,
	ProvideServer,
	ProvidePicker,
	ProvideGroup)

// InitializeApp 初始化
func InitializeApp(addr string) (*App, error) {
	wire.Build(AppSet)
	return &App{}, nil // 返回值没有实际意义，只需符合接口即可
}
