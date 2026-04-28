package account

import (
	"context"
	"fmt"
	"sync"

	"github.com/shoucheng/my-first-agent/infra/config"
)

// ConfigSection 是 account 模块在 config.yaml 里的顶层 key。
const ConfigSection = "account"

// Config 对应 config.yaml 中的 account 段。
type Config struct {
	Source SourceConfig `yaml:"source"`
}

var (
	initOnce sync.Once
	initErr  error
	defSvc   *Service
)

// Init 从全局配置里读 account 段，构造默认账号服务并存入包内单例。
// 多次调用是幂等的：只有第一次会做实际工作。
//
// 返回值是第一次初始化时遇到的错误（如果有）；后续调用会重复返回它。
func Init(ctx context.Context) error {
	initOnce.Do(func() {
		var c Config
		if err := config.Section(ConfigSection, &c); err != nil {
			initErr = fmt.Errorf("account.Init: %w", err)
			return
		}
		s, err := NewService(ctx, c.Source)
		if err != nil {
			initErr = fmt.Errorf("account.Init: %w", err)
			return
		}
		defSvc = s
	})
	return initErr
}

// Default 返回包内默认账号服务。Init 未成功时调用会 panic：
// 这是显式契约 — 默认服务必须在任何业务逻辑之前完成初始化。
func Default() *Service {
	if defSvc == nil {
		panic("account: Default() called before successful Init()")
	}
	return defSvc
}
