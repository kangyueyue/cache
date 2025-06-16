package db

import "gorm.io/gorm"

// Model db
type Model struct {
	gorm.Model
	Key   string
	Value []byte
}
