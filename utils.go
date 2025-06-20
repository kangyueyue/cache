package kamacache

import (
	"os"
	"path/filepath"

	"github.com/zuozikang/cache/consts"
)

// CreateConfigCache 将nacos配置写到更新到本地文件缓存中
func CreateConfigCache(content string) {
	cacheDir := "tmp/nacos/config"
	// 判断是否存在，不存在创建
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		if err = os.MkdirAll(cacheDir, 0755); err != nil {
			return
		}
	}
	cacheFile := filepath.Join(cacheDir, consts.DataId)
	if err := os.WriteFile(cacheFile, []byte(content), 0644); err != nil {
		return
	}
	return
}
