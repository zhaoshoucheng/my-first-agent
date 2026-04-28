package account

import "context"

// Loader 抽象账号集合的来源。
//
// 内置实现：
//   - FileLoader 从文件夹读取所有 *.json
//   - DBLoader   占位（第一版未实现）
type Loader interface {
	Load(ctx context.Context) ([]*Account, error)
}
