package account

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileLoader 从一个目录加载账号：每个 *.json 文件对应一个 Account。
//
// Dir 必须是绝对路径或相对于进程 CWD 的有效路径。
// 不做 ~ 或 $VAR 展开，由配置层自行保证。
type FileLoader struct {
	Dir string
}

// NewFileLoader 创建文件夹账号加载器。
func NewFileLoader(dir string) *FileLoader {
	return &FileLoader{Dir: dir}
}

// Load 实现 Loader 接口。
func (l *FileLoader) Load(_ context.Context) ([]*Account, error) {
	if l.Dir == "" {
		return nil, fmt.Errorf("account.FileLoader: dir is empty")
	}
	entries, err := os.ReadDir(l.Dir)
	if err != nil {
		return nil, fmt.Errorf("account.FileLoader: read dir %q: %w", l.Dir, err)
	}

	accounts := make([]*Account, 0, len(entries))
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(l.Dir, entry.Name())
		acc, err := readAccountFile(path)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[acc.Name]; dup {
			return nil, fmt.Errorf("account.FileLoader: duplicate account name %q (file %s)", acc.Name, path)
		}
		seen[acc.Name] = struct{}{}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

func readAccountFile(path string) (*Account, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("account.FileLoader: read %q: %w", path, err)
	}
	var acc Account
	if err := json.Unmarshal(data, &acc); err != nil {
		return nil, fmt.Errorf("account.FileLoader: parse %q: %w", path, err)
	}
	if err := acc.Validate(); err != nil {
		return nil, fmt.Errorf("account.FileLoader: validate %q: %w", path, err)
	}
	return &acc, nil
}
