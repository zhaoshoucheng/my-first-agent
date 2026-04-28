// Package config 是全局配置基础设施层：把 config/config.yaml 读上来，
// 存成原始 yaml.Node 树，让任何模块按 section 名按需 Decode 自己的那段。
//
// 设计原则：本包对任何 domain 模块零依赖。
// 配置类型由各 domain 模块自己定义、自己解析，infra/config 不替它们做强类型映射，
// 这样新增/修改一个模块的配置不会牵动 infra 层。
//
// 入口约定：
//
//	if err := config.Init("config/config.yaml"); err != nil { ... }
//	var c MyConfig
//	if err := config.Section("mymodule", &c); err != nil { ... }
//
// 放在 infra/ 下而不是 internal/ 下是有意为之：配置是横切关心点，
// 任何包（包括 examples/、cmd/、未来的 server/）都可以 import 进来读，
// 不属于任何单一 domain 的内部细节。
package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	mu     sync.RWMutex
	loaded bool
	root   yaml.Node
)

// Init 读取并解析指定路径的 yaml 配置文件，存入包内全局单例。
// 进程启动时调用一次，之后任何模块都可以通过 Section() 取自己的段。
func Init(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config.Init: read %q: %w", path, err)
	}
	var n yaml.Node
	if err := yaml.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("config.Init: parse %q: %w", path, err)
	}
	mu.Lock()
	root = n
	loaded = true
	mu.Unlock()
	return nil
}

// Section 把顶层名为 key 的 YAML 段解码到 out 指向的结构里。
// 段缺失返回 ErrSectionNotFound — 调用方可据此决定是用默认值还是报错退出。
func Section(key string, out any) error {
	mu.RLock()
	defer mu.RUnlock()
	if !loaded {
		return fmt.Errorf("config.Section(%q): Init() not called yet", key)
	}
	if len(root.Content) == 0 {
		return fmt.Errorf("config.Section(%q): empty document", key)
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("config.Section(%q): root is not a mapping", key)
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == key {
			if err := doc.Content[i+1].Decode(out); err != nil {
				return fmt.Errorf("config.Section(%q): decode: %w", key, err)
			}
			return nil
		}
	}
	return fmt.Errorf("config.Section(%q): %w", key, ErrSectionNotFound)
}

// ErrSectionNotFound 由 Section() 在指定 key 不存在时返回。
var ErrSectionNotFound = fmt.Errorf("section not found")
