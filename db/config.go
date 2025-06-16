package db

import (
	"github.com/BurntSushi/toml"
	"os"
)

// DBConfig 配置
type DBConfig struct {
	host     string `toml:"host"`
	port     string `toml:"port"`
	user     string `toml:"user"`
	password string `toml:"password"`
	dbName   string `toml:"dbName"`
}

// NewDBConfig 创建配置
func NewDBConfig(host, port, user, password, dbName string) *DBConfig {
	return &DBConfig{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		dbName:   dbName,
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
	return c.user + ":" + c.password + "@tcp(" + c.host + ":" + c.port + ")/" + c.dbName + "?charset=utf8mb4&parseTime=True&loc=Local"
}
