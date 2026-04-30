package account

import (
	"context"
	"errors"

	"github.com/shoucheng/my-first-agent/infra/config"
)

// DBLoader 数据库账号加载器（占位）。
type DBLoader struct {
	Config config.DBConfig
}

// NewDBLoader 占位构造。
func NewDBLoader(cfg config.DBConfig) *DBLoader {
	return &DBLoader{Config: cfg}
}

// ErrDBLoaderNotImplemented 表示第一版未实现 db 数据源。
var ErrDBLoaderNotImplemented = errors.New("account.DBLoader: not implemented yet")

// Load 占位实现，返回 not-implemented 错误。
func (l *DBLoader) Load(_ context.Context) ([]*Account, error) {
	return nil, ErrDBLoaderNotImplemented
}
