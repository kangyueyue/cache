package db

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Store 存储
type Store struct {
	db *gorm.DB
}

// NewStore 初始化数据库连接
func NewStore(cfg *DBConfig) (*Store, error) {
	var err error
	dsn := cfg.GetDSN()
	logrus.Infof("dsn:%s", dsn)
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       dsn,
		DefaultStringSize:         256,   // string 类型字段的默认长度
		DisableDatetimePrecision:  true,  // 禁用 datetime 精度，MySQL 5.6 之前的数据库不支持
		DontSupportRenameIndex:    true,  // 重命名索引时采用删除并新建的方式，MySQL 5.7 之前的数据库和 MariaDB 不支持重命名索引
		DontSupportRenameColumn:   true,  // 用 `change` 重命名列，MySQL 8 之前的数据库和 MariaDB 不支持重命名列
		SkipInitializeWithVersion: false, // 根据当前 MySQL 版本自动配置
	}))
	if err != nil {
		logrus.Infof("db init fail,err :%v", err)
		return nil, err
	}
	ss := &Store{db: db}
	// 自动建表
	return ss, ss.db.AutoMigrate(&Model{})
}

// Get From db
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	logrus.Infof("Get key %s From db:", key)
	var model Model
	err := s.db.WithContext(ctx).Where("`key` = ?", key).First(&model).Error
	if err != nil {
		logrus.Infof("get key %s from db err:%v", key, err)
		return nil, fmt.Errorf("get key %s from db err:%v", key, err)
	}
	return model.Value, nil
}

// Set to db
func (s *Store) Set(ctx context.Context, key string, value []byte) error {
	logrus.Infof("Set key %s value %s To db", key, value)

	db := s.db.WithContext(ctx)

	var model Model
	if err := db.Where(Model{Key: key}).Attrs(Model{Value: value}).FirstOrCreate(&model).Error; err != nil {
		logrus.Errorf("Failed to set key %s: %v", key, err)
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}
