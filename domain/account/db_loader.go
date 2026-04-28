package account

import (
	"context"
	"errors"
)

// DBConfig 数据库账号源配置。第一版未实现，仅占位。
type DBConfig struct {
	Driver   string `yaml:"driver"   json:"driver"`
	DSN      string `yaml:"dsn"      json:"dsn"`
	Table    string `yaml:"table"    json:"table"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// DBLoader 数据库账号加载器（占位）。
type DBLoader struct {
	Config DBConfig
}

// NewDBLoader 占位构造。
func NewDBLoader(cfg DBConfig) *DBLoader {
	return &DBLoader{Config: cfg}
}

// ErrDBLoaderNotImplemented 表示第一版未实现 db 数据源。
var ErrDBLoaderNotImplemented = errors.New("account.DBLoader: not implemented yet")

// Load 占位实现，返回 not-implemented 错误。
func (l *DBLoader) Load(_ context.Context) ([]*Account, error) {
	return nil, ErrDBLoaderNotImplemented
}
