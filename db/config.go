package db

import (
	"os"

	"github.com/BurntSushi/toml"
)

// DBConfig 配置
type DBConfig struct {
	Host     string `toml:"host"`
	Port     string `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DbName   string `toml:"dbName"`
}

// NewDBConfig 创建配置
func NewDBConfig(host, port, user, password, dbName string) *DBConfig {
	return &DBConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DbName:   dbName,
	}
}

// NewDBConfigByToml 通过toml配置创建配置
func NewDBConfigByToml(path string) (*DBConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 解析Toml
	var config DBConfig
	if _, err := toml.Decode(string(data), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *DBConfig) GetDSN() string {
	return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + c.Port + ")/" + c.DbName + "?charset=utf8mb4&parseTime=True&loc=Local"
}
