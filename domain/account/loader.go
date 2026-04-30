package account

import (
	"context"
	"fmt"

	"github.com/shoucheng/my-first-agent/infra/config"
)

// Loader 抽象账号集合的来源。
//
// 内置实现：
//   - FileLoader 从文件夹读取所有 *.json
//   - DBLoader   占位（第一版未实现）
type Loader interface {
	Load(ctx context.Context) ([]*Account, error)
}

// NewLoader 根据 Type 选择具体的 Loader 实现。
func NewLoader(c config.SourceConfig) (Loader, error) {
	switch c.Type {
	case config.SourceFile:
		if c.File.Dir == "" {
			return nil, fmt.Errorf("account.SourceConfig: source.file.dir is required when type=file")
		}
		return NewFileLoader(c.File.Dir), nil
	case config.SourceDB:
		return NewDBLoader(c.DB), nil
	case "":
		return nil, fmt.Errorf("account.SourceConfig: source.type is required (file|db)")
	default:
		return nil, fmt.Errorf("account.SourceConfig: source.type %q is not supported", c.Type)
	}
}
