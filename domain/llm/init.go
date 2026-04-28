package llm

import (
	"context"
	"sync"
)

// ConfigSection 是 llm 模块在 config.yaml 里的顶层 key。
const ConfigSection = "llm"

var (
	initOnce sync.Once
	initErr  error
	defSvc   *Service
)

// 当前 llm 段允许缺失（没有必填字段），所以 SectionNotFound 不算错误。
func Init(_ context.Context) error {
	initOnce.Do(func() {
		defSvc = NewService()
	})
	return initErr
}

// Default 返回包内默认 LLM 服务。Init 未成功时调用会 panic：
// 这是显式契约 — 默认服务必须在任何业务逻辑之前完成初始化。
func Default() *Service {
	if defSvc == nil {
		panic("llm: Default() called before successful Init()")
	}
	return defSvc
}
